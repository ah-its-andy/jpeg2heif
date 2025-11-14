package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/utils"
	"github.com/ah-its-andy/jpeg2heif/internal/watcher"
	"github.com/ah-its-andy/jpeg2heif/internal/worker"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Server struct {
	Router *gin.Engine
	cfg    *config.Config
	db     *gorm.DB
	queue  *worker.Queue
	watch  *watcher.Watcher

	jobsMu sync.Mutex
	jobs   map[string]*RebuildJob
}

type RebuildJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // pending/running/done
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Total     int       `json:"total"`
	Indexed   int       `json:"indexed"`
}

func NewServer(cfg *config.Config, db *gorm.DB, q *worker.Queue, w *watcher.Watcher) *Server {
	g := gin.Default()
	s := &Server{Router: g, cfg: cfg, db: db, queue: q, watch: w, jobs: map[string]*RebuildJob{}}
	g.Static("/static", "./static")
	g.GET("/", func(c *gin.Context) { c.File("./static/index.html") })

	api := g.Group("/api")
	api.GET("/files", s.listFiles)
	api.GET("/files/:id", s.getFile)
	api.GET("/tasks", s.listTasks)
	api.GET("/stats", s.getStats)
	api.POST("/rebuild-index", s.rebuildIndex)
	api.GET("/rebuild-status/:id", s.rebuildStatus)
	api.POST("/scan-now", s.scanNow)

	return s
}

func (s *Server) listFiles(c *gin.Context) {
	q := s.db.Model(&db.FileIndex{})
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	limit := parseIntDefault(c.Query("limit"), 50)
	offset := parseIntDefault(c.Query("offset"), 0)
	var rows []db.FileIndex
	var count int64
	q.Count(&count)
	q.Order("updated_at desc").Limit(limit).Offset(offset).Find(&rows)
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": count})
}

func (s *Server) getFile(c *gin.Context) {
	id := c.Param("id")
	var row db.FileIndex
	if err := s.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (s *Server) listTasks(c *gin.Context) {
	limit := parseIntDefault(c.Query("limit"), 100)
	var rows []db.TaskHistory
	s.db.Order("end_time desc").Limit(limit).Find(&rows)
	c.JSON(http.StatusOK, rows)
}

func (s *Server) getStats(c *gin.Context) {
	var total int64
	var success int64
	var failed int64
	s.db.Model(&db.FileIndex{}).Count(&total)
	s.db.Model(&db.FileIndex{}).Where("status = ?", db.StatusSuccess).Count(&success)
	s.db.Model(&db.FileIndex{}).Where("status = ?", db.StatusFailed).Count(&failed)
	var preserved int64
	s.db.Model(&db.FileIndex{}).Where("status = ? AND metadata_preserved = 1", db.StatusSuccess).Count(&preserved)
	rate := 0.0
	if success > 0 {
		rate = float64(preserved) / float64(success)
	}
	c.JSON(http.StatusOK, gin.H{
		"total":                  total,
		"success":                success,
		"failed":                 failed,
		"queue_len":              s.queue.Len(),
		"metadata_preserve_rate": rate,
		"watcher_state": func() string {
			if s.watch.Paused() {
				return "paused"
			} else {
				return "running"
			}
		}(),
	})
}

func (s *Server) rebuildIndex(c *gin.Context) {
	job := &RebuildJob{ID: uuid.NewString(), Status: "running", StartedAt: time.Now()}
	s.jobsMu.Lock()
	s.jobs[job.ID] = job
	s.jobsMu.Unlock()
	go s.runRebuild(job)
	c.JSON(http.StatusOK, gin.H{"job_id": job.ID})
}

func (s *Server) runRebuild(job *RebuildJob) {
	s.watch.Pause()
	defer s.watch.Resume()
	_ = db.WipeIndexes(s.db)
	job.Total = 0
	job.Indexed = 0
	for _, root := range s.cfg.WatchDirs {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(path), ".jpg") && !strings.HasSuffix(strings.ToLower(path), ".jpeg") {
				return nil
			}
			job.Total++
			md5, err := utils.MD5File(path, s.cfg.MD5ChunkSize)
			if err == nil {
				if rec, changed, err := db.UpsertIndex(s.db, path, md5); err == nil {
					if changed {
						s.queue.Enqueue(rec.ID)
					}
					job.Indexed++
				}
			}
			return nil
		})
	}
	job.Status = "done"
	job.EndedAt = time.Now()
}

func (s *Server) rebuildStatus(c *gin.Context) {
	id := c.Param("id")
	s.jobsMu.Lock()
	job, ok := s.jobs[id]
	s.jobsMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (s *Server) scanNow(c *gin.Context) {
	go s.watch.ScanAll(context.Background(), false)
	c.JSON(http.StatusOK, gin.H{"started": true})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
