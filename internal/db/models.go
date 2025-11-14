package db

import (
	"time"
)

// FileIndex represents a file in the index
type FileIndex struct {
	ID                int64     `json:"id"`
	FilePath          string    `json:"file_path"`
	FileMD5           string    `json:"file_md5"`
	Status            string    `json:"status"` // pending, processing, success, failed
	ConverterName     string    `json:"converter_name"`
	MetadataPreserved bool      `json:"metadata_preserved"`
	MetadataSummary   string    `json:"metadata_summary"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// TaskHistory represents a conversion task history entry
type TaskHistory struct {
	ID            int64     `json:"id"`
	FilePath      string    `json:"file_path"`
	ConverterName string    `json:"converter_name"`
	Status        string    `json:"status"` // success, failed
	ErrorMessage  string    `json:"error_message"`
	DurationMs    int64     `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

// Stats represents conversion statistics
type Stats struct {
	TotalFiles      int64 `json:"total_files"`
	SuccessCount    int64 `json:"success_count"`
	FailedCount     int64 `json:"failed_count"`
	PendingCount    int64 `json:"pending_count"`
	ProcessingCount int64 `json:"processing_count"`
}
