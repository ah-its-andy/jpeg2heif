package db

import (
	"fmt"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type FileStatus string

const (
	StatusPending    FileStatus = "pending"
	StatusProcessing FileStatus = "processing"
	StatusSuccess    FileStatus = "success"
	StatusFailed     FileStatus = "failed"
)

type FileIndex struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	FilePath          string     `gorm:"uniqueIndex:idx_files_path;size:2048" json:"file_path"`
	FileMD5           string     `gorm:"index:idx_files_md5;size:64" json:"file_md5"`
	Status            FileStatus `gorm:"index;size:16" json:"status"`
	LastError         *string    `json:"last_error"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ProcessedAt       *time.Time `json:"processed_at"`
	MetadataPreserved bool       `json:"metadata_preserved"`
	MetadataSummary   string     `gorm:"size:2048" json:"metadata_summary"`
}

type TaskHistory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FileIndexID uint      `gorm:"index" json:"file_index_id"`
	Action      string    `gorm:"size:32" json:"action"`
	Status      string    `gorm:"size:16" json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	DurationMs  int64     `json:"duration_ms"`
	Log         string    `gorm:"type:text" json:"log"`
}

func Init(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Silent
	if cfg.LogLevel == "DEBUG" || cfg.LogLevel == "INFO" {
		logLevel = logger.Warn
	}
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.AutoMigrate(&FileIndex{}, &TaskHistory{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func SetStatus(db *gorm.DB, id uint, status FileStatus, lastErr *string) error {
	return db.Model(&FileIndex{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"last_error": lastErr,
	}).Error
}

func UpdateAfterSuccess(db *gorm.DB, id uint, metadataPreserved bool, summary string) error {
	now := time.Now()
	return db.Model(&FileIndex{}).Where("id = ?", id).Updates(map[string]any{
		"status":             StatusSuccess,
		"processed_at":       &now,
		"metadata_preserved": metadataPreserved,
		"metadata_summary":   summary,
	}).Error
}

func InsertTaskHistory(db *gorm.DB, h *TaskHistory) error {
	return db.Create(h).Error
}

func UpsertIndex(db *gorm.DB, path, md5 string) (*FileIndex, bool, error) {
	// returns record, isNewOrUpdated
	var rec FileIndex
	err := db.Where("file_path = ?", path).First(&rec).Error
	if err == nil {
		// existing
		if rec.FileMD5 == md5 && rec.Status == StatusSuccess {
			return &rec, false, nil
		}
		rec.FileMD5 = md5
		rec.Status = StatusPending
		rec.LastError = nil
		if err := db.Save(&rec).Error; err != nil {
			return nil, false, err
		}
		return &rec, true, nil
	}
	// create new
	rec = FileIndex{FilePath: path, FileMD5: md5, Status: StatusPending}
	if err := db.Create(&rec).Error; err != nil {
		return nil, false, err
	}
	return &rec, true, nil
}

func WipeIndexes(db *gorm.DB) error {
	return db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&FileIndex{}).Error
}
