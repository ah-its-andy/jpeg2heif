package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/converter"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/util"
	"github.com/ah-its-andy/jpeg2heif/internal/watcher"
)

// Worker processes file conversion tasks
type Worker struct {
	db           *db.DB
	maxWorkers   int
	quality      int
	preserveMeta bool
	taskQueue    chan *Task
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	md5ChunkSize int
}

// Task represents a conversion task
type Task struct {
	FilePath  string
	Operation string
	Timestamp time.Time
}

// New creates a new worker pool
func New(database *db.DB, maxWorkers, quality int, preserveMeta bool, md5ChunkSize int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		db:           database,
		maxWorkers:   maxWorkers,
		quality:      quality,
		preserveMeta: preserveMeta,
		taskQueue:    make(chan *Task, 1000),
		ctx:          ctx,
		cancel:       cancel,
		md5ChunkSize: md5ChunkSize,
	}
}

// Start starts the worker pool
func (w *Worker) Start() {
	for i := 0; i < w.maxWorkers; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	log.Printf("Started %d conversion workers", w.maxWorkers)
}

// Stop stops the worker pool
func (w *Worker) Stop() {
	w.cancel()
	close(w.taskQueue)
	w.wg.Wait()
	log.Println("All workers stopped")
}

// EnqueueEvent enqueues a file event for processing
func (w *Worker) EnqueueEvent(event watcher.FileEvent) {
	select {
	case w.taskQueue <- &Task{
		FilePath:  event.Path,
		Operation: event.Operation,
		Timestamp: event.Timestamp,
	}:
	case <-w.ctx.Done():
	default:
		log.Printf("Warning: task queue is full, dropping task for %s", event.Path)
	}
}

// worker is the worker goroutine
func (w *Worker) worker(id int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case task, ok := <-w.taskQueue:
			if !ok {
				return
			}
			w.processTask(id, task)
		}
	}
}

// processTask processes a single conversion task
func (w *Worker) processTask(workerID int, task *Task) {
	startTime := time.Now()
	log.Printf("[Worker %d] Processing: %s", workerID, task.FilePath)

	// Calculate file MD5
	md5Hash, err := util.CalculateMD5(task.FilePath, w.md5ChunkSize)
	if err != nil {
		log.Printf("[Worker %d] Failed to calculate MD5 for %s: %v", workerID, task.FilePath, err)
		w.recordFailure(task.FilePath, "", err, time.Since(startTime))
		return
	}

	// Check if file already processed
	existingFile, err := w.db.GetFileIndex(task.FilePath)
	if err != nil {
		log.Printf("[Worker %d] Database error for %s: %v", workerID, task.FilePath, err)
		return
	}

	if existingFile != nil && existingFile.Status == "success" && existingFile.FileMD5 == md5Hash {
		log.Printf("[Worker %d] File already processed: %s", workerID, task.FilePath)
		return
	}

	// Find appropriate converter
	conv, err := converter.FindConverter(task.FilePath, "")
	if err != nil {
		log.Printf("[Worker %d] No converter found for %s: %v", workerID, task.FilePath, err)
		w.recordFailure(task.FilePath, "", err, time.Since(startTime))
		return
	}

	converterName := conv.Name()
	log.Printf("[Worker %d] Using converter: %s for %s", workerID, converterName, task.FilePath)

	// Update status to processing
	fileIndex := &db.FileIndex{
		FilePath:      task.FilePath,
		FileMD5:       md5Hash,
		Status:        "processing",
		ConverterName: converterName,
	}
	if err := w.db.UpsertFileIndex(fileIndex); err != nil {
		log.Printf("[Worker %d] Failed to update file index: %v", workerID, err)
	}

	// Generate output path
	outputPath := w.generateOutputPath(task.FilePath, conv.TargetFormat())

	// Perform conversion
	opts := converter.ConvertOptions{
		Quality:          w.quality,
		PreserveMetadata: w.preserveMeta,
		TempDir:          os.TempDir(), // Use system temp directory
		Timeout:          10 * time.Minute,
	}

	result, err := conv.Convert(w.ctx, task.FilePath, outputPath, opts)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[Worker %d] Conversion failed for %s: %v", workerID, task.FilePath, err)
		w.recordFailure(task.FilePath, converterName, err, duration)
		return
	}

	// Update file index with success
	fileIndex.Status = "success"
	fileIndex.MetadataPreserved = result.MetadataPreserved
	fileIndex.MetadataSummary = result.MetadataSummary
	if err := w.db.UpsertFileIndex(fileIndex); err != nil {
		log.Printf("[Worker %d] Failed to update file index: %v", workerID, err)
	}

	// Record task history
	taskHistory := &db.TaskHistory{
		FilePath:      task.FilePath,
		ConverterName: converterName,
		Status:        "success",
		DurationMs:    duration.Milliseconds(),
	}
	if err := w.db.InsertTaskHistory(taskHistory); err != nil {
		log.Printf("[Worker %d] Failed to insert task history: %v", workerID, err)
	}

	log.Printf("[Worker %d] Successfully converted %s -> %s in %v", workerID, task.FilePath, outputPath, duration)
}

// recordFailure records a conversion failure
func (w *Worker) recordFailure(filePath, converterName string, err error, duration time.Duration) {
	// Calculate MD5 if not already done
	md5Hash, _ := util.CalculateMD5(filePath, w.md5ChunkSize)

	// Update file index
	fileIndex := &db.FileIndex{
		FilePath:      filePath,
		FileMD5:       md5Hash,
		Status:        "failed",
		ConverterName: converterName,
	}
	if dbErr := w.db.UpsertFileIndex(fileIndex); dbErr != nil {
		log.Printf("Failed to update file index: %v", dbErr)
	}

	// Record task history
	taskHistory := &db.TaskHistory{
		FilePath:      filePath,
		ConverterName: converterName,
		Status:        "failed",
		ErrorMessage:  err.Error(),
		DurationMs:    duration.Milliseconds(),
	}
	if dbErr := w.db.InsertTaskHistory(taskHistory); dbErr != nil {
		log.Printf("Failed to insert task history: %v", dbErr)
	}
}

// generateOutputPath generates the output file path
// Source: /a/b/c/photo.jpg -> Output: /a/b/heic/photo.heic
func (w *Worker) generateOutputPath(srcPath, targetFormat string) string {
	dir := filepath.Dir(srcPath)       // /a/b/c
	parentDir := filepath.Dir(dir)     // /a/b
	fileName := filepath.Base(srcPath) // photo.jpg

	// Remove extension and add new one
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	newFileName := fmt.Sprintf("%s.%s", baseName, targetFormat)

	// Create output directory path
	outputDir := filepath.Join(parentDir, targetFormat) // /a/b/heic

	return filepath.Join(outputDir, newFileName) // /a/b/heic/photo.heic
}

// RebuildIndex rebuilds the file index for all watched directories
func (w *Worker) RebuildIndex(watchDirs []string, converterFilter string) error {
	log.Printf("Starting index rebuild (converter filter: %s)", converterFilter)

	var conv converter.Converter
	if converterFilter != "" {
		var ok bool
		conv, ok = converter.Get(converterFilter)
		if !ok {
			return fmt.Errorf("converter not found: %s", converterFilter)
		}
	}

	count := 0
	for _, dir := range watchDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			// Check if file should be processed
			if conv != nil {
				if !conv.CanConvert(path, "") {
					return nil
				}
			} else {
				// Check if any converter can handle it
				if _, err := converter.FindConverter(path, ""); err != nil {
					return nil
				}
			}

			// Calculate MD5
			md5Hash, err := util.CalculateMD5(path, w.md5ChunkSize)
			if err != nil {
				log.Printf("Failed to calculate MD5 for %s: %v", path, err)
				return nil
			}

			// Check if already indexed with same MD5
			existing, err := w.db.GetFileIndex(path)
			if err == nil && existing != nil && existing.FileMD5 == md5Hash {
				return nil
			}

			// Add to index as pending
			converterName := ""
			if conv != nil {
				converterName = conv.Name()
			} else {
				if c, err := converter.FindConverter(path, ""); err == nil {
					converterName = c.Name()
				}
			}

			fileIndex := &db.FileIndex{
				FilePath:      path,
				FileMD5:       md5Hash,
				Status:        "pending",
				ConverterName: converterName,
			}

			if err := w.db.UpsertFileIndex(fileIndex); err != nil {
				log.Printf("Failed to upsert file index for %s: %v", path, err)
				return nil
			}

			count++
			return nil
		})

		if err != nil {
			log.Printf("Error walking directory %s: %v", dir, err)
		}
	}

	log.Printf("Index rebuild completed: %d files indexed", count)
	return nil
}
