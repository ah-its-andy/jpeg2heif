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
	ConsoleOutput string    `json:"console_output"` // 控制台详细输出
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

// Workflow represents a YAML-based conversion workflow
type Workflow struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	YAML        string    `json:"yaml"`
	Enabled     bool      `json:"enabled"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WorkflowRun represents a single execution of a workflow
type WorkflowRun struct {
	ID                int64      `json:"id"`
	WorkflowID        int64      `json:"workflow_id"`
	WorkflowName      string     `json:"workflow_name"`
	FileIndexID       *int64     `json:"file_index_id,omitempty"`
	FilePath          string     `json:"file_path"`
	Status            string     `json:"status"` // pending, running, success, failed
	StartTime         time.Time  `json:"start_time"`
	EndTime           *time.Time `json:"end_time,omitempty"`
	DurationMs        int64      `json:"duration_ms"`
	ExitCode          *int       `json:"exit_code,omitempty"`
	Stdout            string     `json:"stdout"`
	Stderr            string     `json:"stderr"`
	Logs              string     `json:"logs"` // Detailed step-by-step logs
	MetadataPreserved bool       `json:"metadata_preserved"`
	MetadataSummary   string     `json:"metadata_summary"`
	JobParams         string     `json:"job_params"` // JSON of variables used
}

// WorkflowVersion represents a historical version of a workflow
type WorkflowVersion struct {
	ID         int64     `json:"id"`
	WorkflowID int64     `json:"workflow_id"`
	YAML       string    `json:"yaml"`
	EditedBy   string    `json:"edited_by"`
	CreatedAt  time.Time `json:"created_at"`
}
