package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(1) // SQLite only supports one writer
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	db := &DB{conn: conn}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates the database schema
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files_index (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL UNIQUE,
		file_md5 TEXT NOT NULL,
		status TEXT NOT NULL,
		converter_name TEXT,
		metadata_preserved BOOLEAN DEFAULT 0,
		metadata_summary TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_file_path ON files_index(file_path);
	CREATE INDEX IF NOT EXISTS idx_file_md5 ON files_index(file_md5);
	CREATE INDEX IF NOT EXISTS idx_status ON files_index(status);
	
	CREATE TABLE IF NOT EXISTS tasks_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL,
		converter_name TEXT,
		status TEXT NOT NULL,
		error_message TEXT,
		duration_ms INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_task_created ON tasks_history(created_at DESC);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// UpsertFileIndex inserts or updates a file index entry
func (db *DB) UpsertFileIndex(file *FileIndex) error {
	query := `
	INSERT INTO files_index (file_path, file_md5, status, converter_name, metadata_preserved, metadata_summary, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(file_path) DO UPDATE SET
		file_md5 = excluded.file_md5,
		status = excluded.status,
		converter_name = excluded.converter_name,
		metadata_preserved = excluded.metadata_preserved,
		metadata_summary = excluded.metadata_summary,
		updated_at = CURRENT_TIMESTAMP
	`

	result, err := db.conn.Exec(query,
		file.FilePath,
		file.FileMD5,
		file.Status,
		file.ConverterName,
		file.MetadataPreserved,
		file.MetadataSummary,
	)

	if err != nil {
		return err
	}

	if file.ID == 0 {
		id, err := result.LastInsertId()
		if err == nil {
			file.ID = id
		}
	}

	return nil
}

// GetFileIndex retrieves a file index entry by path
func (db *DB) GetFileIndex(filePath string) (*FileIndex, error) {
	query := `SELECT id, file_path, file_md5, status, converter_name, metadata_preserved, metadata_summary, created_at, updated_at
	          FROM files_index WHERE file_path = ?`

	file := &FileIndex{}
	err := db.conn.QueryRow(query, filePath).Scan(
		&file.ID,
		&file.FilePath,
		&file.FileMD5,
		&file.Status,
		&file.ConverterName,
		&file.MetadataPreserved,
		&file.MetadataSummary,
		&file.CreatedAt,
		&file.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return file, err
}

// ListFiles lists file index entries with pagination and filtering
func (db *DB) ListFiles(status string, limit, offset int) ([]*FileIndex, error) {
	query := `SELECT id, file_path, file_md5, status, converter_name, metadata_preserved, metadata_summary, created_at, updated_at
	          FROM files_index`

	args := []interface{}{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := []*FileIndex{}
	for rows.Next() {
		file := &FileIndex{}
		err := rows.Scan(
			&file.ID,
			&file.FilePath,
			&file.FileMD5,
			&file.Status,
			&file.ConverterName,
			&file.MetadataPreserved,
			&file.MetadataSummary,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, rows.Err()
}

// InsertTaskHistory adds a task history entry
func (db *DB) InsertTaskHistory(task *TaskHistory) error {
	query := `INSERT INTO tasks_history (file_path, converter_name, status, error_message, duration_ms)
	          VALUES (?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		task.FilePath,
		task.ConverterName,
		task.Status,
		task.ErrorMessage,
		task.DurationMs,
	)

	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err == nil {
		task.ID = id
	}

	return nil
}

// ListTasks lists task history entries with pagination
func (db *DB) ListTasks(limit, offset int) ([]*TaskHistory, error) {
	query := `SELECT id, file_path, converter_name, status, error_message, duration_ms, created_at
	          FROM tasks_history
	          ORDER BY created_at DESC
	          LIMIT ? OFFSET ?`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []*TaskHistory{}
	for rows.Next() {
		task := &TaskHistory{}
		err := rows.Scan(
			&task.ID,
			&task.FilePath,
			&task.ConverterName,
			&task.Status,
			&task.ErrorMessage,
			&task.DurationMs,
			&task.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// GetStats retrieves conversion statistics
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{}

	query := `
	SELECT 
		COUNT(*) as total,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
		SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
		SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END) as processing
	FROM files_index
	`

	err := db.conn.QueryRow(query).Scan(
		&stats.TotalFiles,
		&stats.SuccessCount,
		&stats.FailedCount,
		&stats.PendingCount,
		&stats.ProcessingCount,
	)

	return stats, err
}

// DeleteFileIndex deletes a file index entry
func (db *DB) DeleteFileIndex(filePath string) error {
	query := `DELETE FROM files_index WHERE file_path = ?`
	_, err := db.conn.Exec(query, filePath)
	return err
}

// ClearIndex clears all file index entries
func (db *DB) ClearIndex() error {
	_, err := db.conn.Exec(`DELETE FROM files_index`)
	return err
}
