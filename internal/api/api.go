package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/watcher"
	"github.com/ah-its-andy/jpeg2heif/internal/worker"
	"github.com/google/uuid"
)

// Server represents the API server
type Server struct {
	db            *db.DB
	worker        *worker.Worker
	watcher       *watcher.Watcher
	watchDirs     []string
	rebuildJobs   map[string]*RebuildJob
	rebuildJobsMu sync.RWMutex
}

// RebuildJob represents a rebuild index job
type RebuildJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // running, completed, failed
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// New creates a new API server
func New(database *db.DB, w *worker.Worker, watcher *watcher.Watcher, watchDirs []string) *Server {
	return &Server{
		db:          database,
		worker:      w,
		watcher:     watcher,
		watchDirs:   watchDirs,
		rebuildJobs: make(map[string]*RebuildJob),
	}
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/files/", s.handleFileDetail)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskDetail)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/converters", s.handleConverters)
	mux.HandleFunc("/api/converters/", s.handleConverterDetail)
	mux.HandleFunc("/api/rebuild-index", s.handleRebuildIndex)
	mux.HandleFunc("/api/rebuild-status/", s.handleRebuildStatus)
	mux.HandleFunc("/api/scan-now", s.handleScanNow)

	// Workflow API routes
	mux.HandleFunc("/api/workflows", s.handleWorkflows)
	mux.HandleFunc("/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/validate") {
			s.handleWorkflowValidate(w, r)
		} else if strings.Contains(path, "/run") {
			s.handleWorkflowRun(w, r)
		} else if strings.Contains(path, "/runs/") {
			s.handleWorkflowRunDetail(w, r)
		} else if strings.HasSuffix(path, "/runs") {
			s.handleWorkflowRuns(w, r)
		} else {
			s.handleWorkflowDetail(w, r)
		}
	})

	// Static files
	mux.Handle("/", http.FileServer(http.Dir("static")))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting HTTP server on %s", addr)
	return http.ListenAndServe(addr, s.corsMiddleware(mux))
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleFiles handles GET /api/files
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 50
	}

	files, err := s.db.ListFiles(status, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, files)
}

// handleFileDetail handles GET /api/files/{id}
func (s *Server) handleFileDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	id := r.URL.Path[len("/api/files/"):]
	if id == "" {
		http.Error(w, "File ID required", http.StatusBadRequest)
		return
	}

	// For simplicity, we'll search by file path
	// In a real implementation, you might want to search by ID
	files, err := s.db.ListFiles("", 1000, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, file := range files {
		if strconv.FormatInt(file.ID, 10) == id {
			respondJSON(w, file)
			return
		}
	}

	http.Error(w, "File not found", http.StatusNotFound)
}

// handleTasks handles GET /api/tasks
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 100
	}

	tasks, err := s.db.ListTasks(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, tasks)
}

// handleTaskDetail handles GET /api/tasks/{id}
func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	id := r.URL.Path[len("/api/tasks/"):]
	taskID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	task, err := s.db.GetTaskByID(taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	respondJSON(w, task)
}

// handleStats handles GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, stats)
}

// handleRebuildIndex handles POST /api/rebuild-index
func (s *Server) handleRebuildIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Converter string `json:"converter"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty or invalid, just use empty converter filter
		req.Converter = ""
	}

	// Create rebuild job
	jobID := uuid.New().String()
	job := &RebuildJob{
		ID:        jobID,
		Status:    "running",
		StartTime: time.Now(),
	}

	s.rebuildJobsMu.Lock()
	s.rebuildJobs[jobID] = job
	s.rebuildJobsMu.Unlock()

	// Start rebuild in background
	go func() {
		err := s.worker.RebuildIndex(s.watchDirs, req.Converter)

		s.rebuildJobsMu.Lock()
		defer s.rebuildJobsMu.Unlock()

		job.EndTime = time.Now()
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
		} else {
			job.Status = "completed"
		}
	}()

	respondJSON(w, map[string]string{"job_id": jobID})
}

// handleRebuildStatus handles GET /api/rebuild-status/{job_id}
func (s *Server) handleRebuildStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Path[len("/api/rebuild-status/"):]
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	s.rebuildJobsMu.RLock()
	job, exists := s.rebuildJobs[jobID]
	s.rebuildJobsMu.RUnlock()

	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	respondJSON(w, job)
}

// handleScanNow handles POST /api/scan-now
func (s *Server) handleScanNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.watcher.TriggerScan()
	respondJSON(w, map[string]string{"status": "scan triggered"})
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}
