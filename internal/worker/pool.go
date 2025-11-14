package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"gorm.io/gorm"
)

type Pool struct {
	cfg   *config.Config
	db    *gorm.DB
	queue *Queue
	conv  *Converter
}

func NewPool(cfg *config.Config, db *gorm.DB, q *Queue, conv *Converter) *Pool {
	return &Pool{cfg: cfg, db: db, queue: q, conv: conv}
}

func (p *Pool) Run(ctx context.Context) {
	for i := 0; i < p.cfg.MaxWorkers; i++ {
		go p.worker(ctx, i)
	}
}

func (p *Pool) worker(ctx context.Context, idx int) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-p.queue.Chan():
			p.handle(ctx, id)
		}
	}
}

func (p *Pool) handle(ctx context.Context, id uint) {
	defer p.queue.Dequeued(id)
	var rec db.FileIndex
	if err := p.db.First(&rec, id).Error; err != nil {
		log.Printf("worker: load file %d: %v", id, err)
		return
	}
	if err := db.SetStatus(p.db, rec.ID, db.StatusProcessing, nil); err != nil {
		log.Printf("worker: set processing: %v", err)
	}
	start := time.Now()
	preserved, summary, logs, err := p.conv.Convert(ctx, &rec)
	status := db.StatusSuccess
	var lastErr *string
	if err != nil {
		status = db.StatusFailed
		e := err.Error()
		lastErr = &e
		log.Printf("convert failed for %s: %v", rec.FilePath, err)
	}

	if status == db.StatusSuccess {
		if err := db.UpdateAfterSuccess(p.db, rec.ID, preserved, summary); err != nil {
			log.Printf("update after success failed: %v", err)
		}
	} else {
		if err := db.SetStatus(p.db, rec.ID, status, lastErr); err != nil {
			log.Printf("set failed status failed: %v", err)
		}
	}
	h := &db.TaskHistory{
		FileIndexID: rec.ID,
		Action:      "convert",
		Status:      string(status),
		StartTime:   start,
		EndTime:     time.Now(),
		DurationMs:  time.Since(start).Milliseconds(),
		Log:         fmt.Sprintf("%s\nsummary: %s", logs, summary),
	}
	if err := db.InsertTaskHistory(p.db, h); err != nil {
		log.Printf("insert task history failed: %v", err)
	}
}

func (p *Pool) Drain(ctx context.Context) {
	// Let workers finish; we can sleep for a short period or track waitgroup.
	t := time.NewTimer(5 * time.Second)
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
