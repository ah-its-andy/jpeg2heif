package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/api"
	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/watcher"
	"github.com/ah-its-andy/jpeg2heif/internal/worker"
)

func main() {
	cfg := config.Load()
	log.Printf("starting jpeg2heif watcher on port %d, db=%s, watch=%v", cfg.HTTPPort, cfg.DBPath, cfg.WatchDirs)

	dbConn, err := db.Init(cfg)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		_sqlDB, _ := dbConn.DB()
		_ = _sqlDB.Close()
	}()

	// Job queue and workers
	queue := worker.NewQueue(cfg.MaxWorkers)
	conv := worker.NewConverter(cfg, dbConn)
	wp := worker.NewPool(cfg, dbConn, queue, conv)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go wp.Run(ctx)

	// Watcher
	w, err := watcher.NewRecursiveWatcher(cfg, dbConn, queue)
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Close()
	go func() {
		if err := w.Start(ctx); err != nil {
			log.Printf("watcher error: %v", err)
		}
	}()

	// Initial full scan
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := w.ScanAll(ctx, false /*rebuild*/); err != nil {
			log.Printf("initial scan error: %v", err)
		}
	}()

	// API server
	server := api.NewServer(cfg, dbConn, queue, w)
	httpSrv := &http.Server{Addr: cfg.HTTPAddr(), Handler: server.Router}
	go func() {
		log.Printf("http server listening on %s", cfg.HTTPAddr())
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	s := <-sigCh
	log.Printf("received signal %s, shutting down...", s)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()
	w.Pause()
	queue.StopAccepting()
	_ = httpSrv.Shutdown(shutdownCtx)
	wp.Drain(shutdownCtx)
	log.Printf("shutdown complete")
}
