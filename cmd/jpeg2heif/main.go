package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ah-its-andy/jpeg2heif/internal/api"
	"github.com/ah-its-andy/jpeg2heif/internal/converter"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/util"
	"github.com/ah-its-andy/jpeg2heif/internal/watcher"
	"github.com/ah-its-andy/jpeg2heif/internal/worker"
)

func main() {
	log.Println("Starting JPEG2HEIF...")

	// Load configuration
	cfg := util.LoadConfig()
	log.Printf("Configuration loaded:")
	log.Printf("  Watch Dirs: %v", cfg.WatchDirs)
	log.Printf("  DB Path: %s", cfg.DBPath)
	log.Printf("  HTTP Port: %d", cfg.HTTPPort)
	log.Printf("  Max Workers: %d", cfg.MaxWorkers)
	log.Printf("  Quality: %d", cfg.ConvertQuality)
	log.Printf("  Preserve Metadata: %t", cfg.PreserveMetadata)

	// Check external tools
	checkExternalTools()

	// Register builtin converters based on environment variable
	converter.RegisterBuiltinConverters()

	// Initialize database
	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	log.Println("Database initialized")

	// Load workflow converters from database
	if err := converter.LoadWorkflowConverters(database); err != nil {
		log.Printf("Warning: failed to load workflow converters: %v", err)
	}

	// Create watcher
	w, err := watcher.New(cfg.WatchDirs, cfg.MetadataStabilityDelay, cfg.PollInterval)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Create worker pool
	wrk := worker.New(database, cfg.MaxWorkers, cfg.ConvertQuality, cfg.PreserveMetadata, cfg.MD5ChunkSize)

	// Start worker pool
	wrk.Start()

	// Start watcher
	if err := w.Start(); err != nil {
		log.Fatalf("Failed to start watcher: %v", err)
	}

	// Connect watcher to worker
	go func() {
		for event := range w.Events() {
			wrk.EnqueueEvent(event)
		}
	}()

	// Create and start API server
	server := api.New(database, wrk, w, cfg.WatchDirs)
	go func() {
		if err := server.Start(cfg.HTTPPort); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	log.Println("JPEG2HEIF is running")
	log.Printf("Web UI available at: http://localhost:%d/", cfg.HTTPPort)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Stop components
	w.Stop()
	wrk.Stop()

	log.Println("Shutdown complete")
}

// checkExternalTools checks if required external tools are available
func checkExternalTools() {
	tools := []string{"heif-enc", "exiftool"}

	log.Println("Checking external tools:")
	for _, name := range tools {
		if _, err := exec.LookPath(name); err != nil {
			log.Printf("  ⚠️  %s: NOT FOUND (required for conversion)", name)
		} else {
			log.Printf("  ✅ %s: found", name)
		}
	}
}
