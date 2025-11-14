package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Operation string
	Timestamp time.Time
}

// Watcher monitors directories for file changes
type Watcher struct {
	fsWatcher      *fsnotify.Watcher
	watchDirs      []string
	fileQueue      chan FileEvent
	stabilityDelay time.Duration
	pollInterval   time.Duration
	pendingFiles   map[string]*pendingFile
	pendingMu      sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	watchedDirs    map[string]bool
	watchedDirsMu  sync.RWMutex
}

type pendingFile struct {
	path        string
	lastSize    int64
	lastModTime time.Time
	firstSeen   time.Time
}

// New creates a new watcher
func New(watchDirs []string, stabilityDelay, pollInterval time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		fsWatcher:      fsw,
		watchDirs:      watchDirs,
		fileQueue:      make(chan FileEvent, 1000),
		stabilityDelay: stabilityDelay,
		pollInterval:   pollInterval,
		pendingFiles:   make(map[string]*pendingFile),
		ctx:            ctx,
		cancel:         cancel,
		watchedDirs:    make(map[string]bool),
	}

	return w, nil
}

// Start starts the watcher
func (w *Watcher) Start() error {
	// Add initial watch directories recursively
	for _, dir := range w.watchDirs {
		if err := w.addRecursive(dir); err != nil {
			log.Printf("Warning: failed to add watch directory %s: %v", dir, err)
		}
	}

	// Start event processor
	go w.processEvents()

	// Start stability checker
	go w.checkStability()

	// Start periodic scan
	go w.periodicScan()

	log.Printf("Watcher started, monitoring %d directories", len(w.watchedDirs))
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	w.cancel()
	close(w.fileQueue)
	return w.fsWatcher.Close()
}

// Events returns the file event channel
func (w *Watcher) Events() <-chan FileEvent {
	return w.fileQueue
}

// addRecursive adds a directory and all its subdirectories to the watcher
func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if err := w.addDir(path); err != nil {
				log.Printf("Warning: failed to watch directory %s: %v", path, err)
			}
		}

		return nil
	})
}

// addDir adds a single directory to the watcher
func (w *Watcher) addDir(path string) error {
	w.watchedDirsMu.Lock()
	defer w.watchedDirsMu.Unlock()

	if w.watchedDirs[path] {
		return nil
	}

	if err := w.fsWatcher.Add(path); err != nil {
		return err
	}

	w.watchedDirs[path] = true
	return nil
}

// processEvents processes file system events
func (w *Watcher) processEvents() {
	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// handleEvent handles a single file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Check if it's a directory creation - add to watch list
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := w.addRecursive(event.Name); err != nil {
				log.Printf("Failed to add new directory to watch: %v", err)
			}
			return
		}
	}

	// Only process file creation and modification
	if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
		return
	}

	// Skip directories
	if info, err := os.Stat(event.Name); err != nil || info.IsDir() {
		return
	}

	// Check if it's a media file we care about
	if !w.isMediaFile(event.Name) {
		return
	}

	// Add to pending files for stability check
	w.pendingMu.Lock()
	if _, exists := w.pendingFiles[event.Name]; !exists {
		info, err := os.Stat(event.Name)
		if err != nil {
			w.pendingMu.Unlock()
			return
		}

		w.pendingFiles[event.Name] = &pendingFile{
			path:        event.Name,
			lastSize:    info.Size(),
			lastModTime: info.ModTime(),
			firstSeen:   time.Now(),
		}
	}
	w.pendingMu.Unlock()
}

// checkStability checks if pending files are stable and ready for processing
func (w *Watcher) checkStability() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.pendingMu.Lock()
			now := time.Now()

			for path, pf := range w.pendingFiles {
				// Check if file has been pending long enough
				if now.Sub(pf.firstSeen) < w.stabilityDelay {
					continue
				}

				// Check current file state
				info, err := os.Stat(path)
				if err != nil {
					// File disappeared, remove from pending
					delete(w.pendingFiles, path)
					continue
				}

				// Check if size and mod time are stable
				if info.Size() == pf.lastSize && info.ModTime().Equal(pf.lastModTime) {
					// File is stable, send to queue
					select {
					case w.fileQueue <- FileEvent{
						Path:      path,
						Operation: "create",
						Timestamp: now,
					}:
						delete(w.pendingFiles, path)
					default:
						log.Printf("Warning: file queue is full, skipping %s", path)
					}
				} else {
					// File is still changing, update last known state
					pf.lastSize = info.Size()
					pf.lastModTime = info.ModTime()
				}
			}

			w.pendingMu.Unlock()
		}
	}
}

// periodicScan performs a periodic scan of watch directories
func (w *Watcher) periodicScan() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Do an initial scan
	w.scanDirectories()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.scanDirectories()
		}
	}
}

// scanDirectories scans all watch directories for files
func (w *Watcher) scanDirectories() {
	log.Println("Starting periodic directory scan")

	for _, dir := range w.watchDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Continue on error
			}

			if info.IsDir() {
				return nil
			}

			if !w.isMediaFile(path) {
				return nil
			}

			// Check if file is already pending
			w.pendingMu.Lock()
			_, pending := w.pendingFiles[path]
			w.pendingMu.Unlock()

			if pending {
				return nil
			}

			// Add to queue directly (periodic scan assumes files are stable)
			select {
			case w.fileQueue <- FileEvent{
				Path:      path,
				Operation: "scan",
				Timestamp: time.Now(),
			}:
			default:
				log.Printf("Warning: file queue is full during scan")
			}

			return nil
		})

		if err != nil {
			log.Printf("Error scanning directory %s: %v", dir, err)
		}
	}

	log.Println("Periodic directory scan completed")
}

// isMediaFile checks if a file is a media file we should process
func (w *Watcher) isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	mediaExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff"}

	for _, mediaExt := range mediaExtensions {
		if ext == mediaExt {
			return true
		}
	}

	return false
}

// TriggerScan triggers an immediate scan
func (w *Watcher) TriggerScan() {
	go w.scanDirectories()
}
