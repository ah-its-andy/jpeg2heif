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
	log.Printf("[Worker %d] ========== Processing Task ==========", workerID)
	log.Printf("[Worker %d] File: %s", workerID, task.FilePath)
	log.Printf("[Worker %d] Operation: %s", workerID, task.Operation)

	// Calculate file MD5
	log.Printf("[Worker %d] Calculating MD5 hash...", workerID)
	md5Hash, err := util.CalculateMD5(task.FilePath, w.md5ChunkSize)
	if err != nil {
		log.Printf("[Worker %d] ❌ Failed to calculate MD5: %v", workerID, err)
		w.recordFailure(task.FilePath, "", err, time.Since(startTime), "")
		return
	}
	log.Printf("[Worker %d] MD5: %s", workerID, md5Hash)

	// Check if file already processed
	log.Printf("[Worker %d] Checking if file already processed...", workerID)
	existingFile, err := w.db.GetFileIndex(task.FilePath)
	if err != nil {
		log.Printf("[Worker %d] ❌ Database error: %v", workerID, err)
		w.recordFailure(task.FilePath, "", fmt.Errorf("database error: %w", err), time.Since(startTime), "")
		return
	}

	if existingFile != nil && existingFile.Status == "success" && existingFile.FileMD5 == md5Hash {
		log.Printf("[Worker %d] ⏭️  File already processed successfully (MD5 match)", workerID)
		// Record as skipped task
		w.recordSkipped(task.FilePath, existingFile.ConverterName, time.Since(startTime), "File already processed successfully")
		return
	}

	if existingFile != nil {
		log.Printf("[Worker %d] Existing file index: status=%s, md5=%s", workerID, existingFile.Status, existingFile.FileMD5)
	} else {
		log.Printf("[Worker %d] No existing file index found", workerID)
	}

	// Find appropriate converter
	log.Printf("[Worker %d] Finding converter...", workerID)
	conv, err := converter.FindConverter(task.FilePath, "")
	if err != nil {
		log.Printf("[Worker %d] ❌ No converter found: %v", workerID, err)
		w.recordFailure(task.FilePath, "", err, time.Since(startTime), "")
		return
	}

	converterName := conv.Name()
	log.Printf("[Worker %d] ✅ Using converter: %s", workerID, converterName)
	log.Printf("[Worker %d] Target format: %s", workerID, conv.TargetFormat())

	// Log if this is a workflow converter
	if strings.HasPrefix(converterName, "workflow:") {
		log.Printf("[Worker %d] 📋 This is a WORKFLOW converter", workerID)
	} else {
		log.Printf("[Worker %d] 🔧 This is a BUILTIN converter", workerID)
	}

	// Update status to processing
	log.Printf("[Worker %d] Updating file index to 'processing' status...", workerID)
	fileIndex := &db.FileIndex{
		FilePath:      task.FilePath,
		FileMD5:       md5Hash,
		Status:        "processing",
		ConverterName: converterName,
	}
	if err := w.db.UpsertFileIndex(fileIndex); err != nil {
		log.Printf("[Worker %d] ⚠️  Failed to update file index: %v", workerID, err)
	}

	// Generate output path
	outputPath := w.generateOutputPath(task.FilePath, conv.TargetFormat())
	log.Printf("[Worker %d] Output path: %s", workerID, outputPath)

	// Perform conversion
	log.Printf("[Worker %d] Starting conversion...", workerID)
	opts := converter.ConvertOptions{
		Quality:          w.quality,
		PreserveMetadata: w.preserveMeta,
		TempDir:          os.TempDir(),
		Timeout:          10 * time.Minute,
	}
	log.Printf("[Worker %d] Conversion options: quality=%d, preserveMetadata=%v", workerID, w.quality, w.preserveMeta)

	result, err := conv.Convert(w.ctx, task.FilePath, outputPath, opts)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[Worker %d] ❌ Conversion failed (duration: %v): %v", workerID, duration, err)
		// Pass the conversion log even on failure for detailed error information
		w.recordFailure(task.FilePath, converterName, err, duration, result.ConversionLog)
		return
	}

	log.Printf("[Worker %d] ✅ Conversion successful (duration: %v)", workerID, duration)
	log.Printf("[Worker %d] Metadata preserved: %v", workerID, result.MetadataPreserved)
	if result.MetadataSummary != "" {
		log.Printf("[Worker %d] Metadata summary: %s", workerID, result.MetadataSummary)
	}

	// Update file index with success
	log.Printf("[Worker %d] Updating file index to 'success' status...", workerID)
	fileIndex.Status = "success"
	fileIndex.MetadataPreserved = result.MetadataPreserved
	fileIndex.MetadataSummary = result.MetadataSummary
	if err := w.db.UpsertFileIndex(fileIndex); err != nil {
		log.Printf("[Worker %d] ⚠️  Failed to update file index: %v", workerID, err)
	}

	// Record task history with console output
	log.Printf("[Worker %d] Recording task history...", workerID)
	taskHistory := &db.TaskHistory{
		FilePath:      task.FilePath,
		ConverterName: converterName,
		Status:        "success",
		DurationMs:    duration.Milliseconds(),
		ConsoleOutput: result.ConversionLog,
	}
	if err := w.db.InsertTaskHistory(taskHistory); err != nil {
		log.Printf("[Worker %d] ⚠️  Failed to insert task history: %v", workerID, err)
	}

	log.Printf("[Worker %d] ✅ Task completed successfully: %s -> %s", workerID, task.FilePath, outputPath)
	log.Printf("[Worker %d] ==========================================", workerID)
}

// recordFailure records a conversion failure
func (w *Worker) recordFailure(filePath, converterName string, err error, duration time.Duration, conversionLog string) {
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

	// Build detailed console output
	consoleOutput := conversionLog
	if consoleOutput == "" {
		consoleOutput = fmt.Sprintf("Error: %v", err)
	} else {
		// Append error summary at the end if we have conversion log
		consoleOutput = fmt.Sprintf("%s\n\n╔═══════════════════════════════════════════════════════════╗\n║ FINAL ERROR SUMMARY\n╚═══════════════════════════════════════════════════════════╝\nError: %v", conversionLog, err)
	}

	// Record task history with detailed console output
	taskHistory := &db.TaskHistory{
		FilePath:      filePath,
		ConverterName: converterName,
		Status:        "failed",
		ErrorMessage:  err.Error(),
		DurationMs:    duration.Milliseconds(),
		ConsoleOutput: consoleOutput,
	}
	if dbErr := w.db.InsertTaskHistory(taskHistory); dbErr != nil {
		log.Printf("Failed to insert task history: %v", dbErr)
	}
}

// recordSkipped records a skipped task (e.g., file already processed)
func (w *Worker) recordSkipped(filePath, converterName string, duration time.Duration, reason string) {
	// Record task history as skipped
	taskHistory := &db.TaskHistory{
		FilePath:      filePath,
		ConverterName: converterName,
		Status:        "skipped",
		ErrorMessage:  "",
		DurationMs:    duration.Milliseconds(),
		ConsoleOutput: fmt.Sprintf("Task skipped: %s", reason),
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
	log.Printf("Watched directories: %v", watchDirs)

	// Clear existing index
	log.Printf("Clearing existing file index...")
	if err := w.db.ClearIndex(); err != nil {
		log.Printf("⚠️  Warning: failed to clear index: %v", err)
		return fmt.Errorf("failed to clear index: %w", err)
	}
	log.Printf("✅ File index cleared")

	var conv converter.Converter
	if converterFilter != "" {
		var ok bool
		conv, ok = converter.Get(converterFilter)
		if !ok {
			return fmt.Errorf("converter not found: %s", converterFilter)
		}
		log.Printf("Using converter filter: %s", conv.Name())
	} else {
		log.Printf("No converter filter - will check all available converters")
	}

	count := 0
	scannedFiles := 0
	skippedDirs := 0

	for _, dir := range watchDirs {
		log.Printf("Scanning directory: %s", dir)
		dirCount := 0

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Error accessing path %s: %v", path, err)
				return nil
			}

			if info.IsDir() {
				if path != dir {
					skippedDirs++
					log.Printf("  [DIR] %s", path)
				}
				return nil
			}

			scannedFiles++
			log.Printf("  [FILE] Checking: %s (size: %d bytes)", path, info.Size())

			// Check if file should be processed
			if conv != nil {
				if !conv.CanConvert(path, "") {
					log.Printf("    ❌ Skipped: converter '%s' cannot convert this file", conv.Name())
					return nil
				}
				log.Printf("    ✅ Converter '%s' can convert this file", conv.Name())
			} else {
				// Check if any converter can handle it
				foundConv, err := converter.FindConverter(path, "")
				if err != nil {
					log.Printf("    ❌ Skipped: no converter found for this file - %v", err)
					return nil
				}
				log.Printf("    ✅ Found converter: %s", foundConv.Name())
			}

			// Calculate MD5
			log.Printf("    📊 Calculating MD5...")
			md5Hash, err := util.CalculateMD5(path, w.md5ChunkSize)
			if err != nil {
				log.Printf("    ❌ Failed to calculate MD5: %v", err)
				return nil
			}
			log.Printf("    📊 MD5: %s", md5Hash)

			// Check if already indexed with same MD5
			existing, err := w.db.GetFileIndex(path)
			if err == nil && existing != nil && existing.FileMD5 == md5Hash {
				log.Printf("    ⏭️  Skipped: already indexed with same MD5 (status: %s)", existing.Status)
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

			log.Printf("    💾 Adding to index (converter: %s, status: pending)", converterName)
			fileIndex := &db.FileIndex{
				FilePath:      path,
				FileMD5:       md5Hash,
				Status:        "pending",
				ConverterName: converterName,
			}

			if err := w.db.UpsertFileIndex(fileIndex); err != nil {
				log.Printf("    ❌ Failed to upsert file index: %v", err)
				return nil
			}

			log.Printf("    ✅ Successfully indexed")
			count++
			dirCount++
			return nil
		})

		if err != nil {
			log.Printf("Error walking directory %s: %v", dir, err)
		}

		log.Printf("Completed scanning %s: %d files indexed from this directory", dir, dirCount)
	}

	log.Printf("Index rebuild completed:")
	log.Printf("  - Total files scanned: %d", scannedFiles)
	log.Printf("  - Total directories: %d", skippedDirs)
	log.Printf("  - Files indexed: %d", count)
	log.Printf("  - Files skipped: %d", scannedFiles-count)

	return nil
}
