package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/utils"
	"github.com/ah-its-andy/jpeg2heif/internal/worker"
	"github.com/fsnotify/fsnotify"
	"gorm.io/gorm"
)

type Watcher struct {
	cfg    *config.Config
	db     *gorm.DB
	queue  *worker.Queue
	w      *fsnotify.Watcher
	roots  []string
	mu     sync.Mutex
	paused bool
}

func NewRecursiveWatcher(cfg *config.Config, db *gorm.DB, q *worker.Queue) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	wr := &Watcher{cfg: cfg, db: db, queue: q, w: w, roots: cfg.WatchDirs}
	return wr, nil
}

func (wr *Watcher) Start(ctx context.Context) error {
	// Register roots and all subdirs
	if err := wr.registerAll(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-wr.w.Events:
			wr.handleEvent(ev)
		case err := <-wr.w.Errors:
			log.Printf("watcher error: %v", err)
		}
	}
}

func (wr *Watcher) Close() error { return wr.w.Close() }

func (wr *Watcher) Pause()       { wr.mu.Lock(); wr.paused = true; wr.mu.Unlock() }
func (wr *Watcher) Resume()      { wr.mu.Lock(); wr.paused = false; wr.mu.Unlock() }
func (wr *Watcher) Paused() bool { wr.mu.Lock(); defer wr.mu.Unlock(); return wr.paused }

func (wr *Watcher) registerAll() error {
	for _, root := range wr.roots {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				_ = wr.w.Add(path)
			}
			return nil
		})
	}
	return nil
}

func (wr *Watcher) handleEvent(ev fsnotify.Event) {
	// New directories
	if ev.Op&(fsnotify.Create) != 0 {
		fi, err := os.Stat(ev.Name)
		if err == nil && fi.IsDir() {
			// add new dir and subdirs
			_ = filepath.WalkDir(ev.Name, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					_ = wr.w.Add(path)
				}
				return nil
			})
			return
		}
	}
	if wr.Paused() {
		return
	}
	// New or modified JPEG
	ext := strings.ToLower(filepath.Ext(ev.Name))
	if ext == ".jpg" || ext == ".jpeg" {
		go func(path string) {
			time.Sleep(time.Duration(wr.cfg.MetadataStabilityDelay) * time.Second)
			if err := wr.indexAndEnqueue(path); err != nil {
				log.Printf("index/enqueue error: %v", err)
			}
		}(ev.Name)
	}
}

func (wr *Watcher) indexAndEnqueue(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	md5, err := utils.MD5File(path, wr.cfg.MD5ChunkSize)
	if err != nil {
		return err
	}
	rec, changed, err := db.UpsertIndex(wr.db, path, md5)
	if err != nil {
		return err
	}
	if changed {
		wr.queue.Enqueue(rec.ID)
	}
	return nil
}

func (wr *Watcher) ScanAll(ctx context.Context, rebuild bool) error {
	for _, root := range wr.roots {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !utils.IsJPEG(path) {
				return nil
			}
			if rebuild || true {
				if err := wr.indexAndEnqueue(path); err != nil {
					log.Printf("scan index error: %v", err)
				}
			}
			return nil
		})
	}
	return nil
}
