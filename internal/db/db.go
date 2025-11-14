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
		console_output TEXT,
		duration_ms INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_task_created ON tasks_history(created_at DESC);
	
	CREATE TABLE IF NOT EXISTS workflows (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		yaml TEXT NOT NULL,
		enabled BOOLEAN DEFAULT 1,
		created_by TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_workflow_name ON workflows(name);
	CREATE INDEX IF NOT EXISTS idx_workflow_enabled ON workflows(enabled);
	
	CREATE TABLE IF NOT EXISTS workflow_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workflow_id INTEGER NOT NULL,
		workflow_name TEXT NOT NULL,
		file_index_id INTEGER,
		file_path TEXT NOT NULL,
		status TEXT NOT NULL,
		start_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		end_time TIMESTAMP,
		duration_ms INTEGER,
		exit_code INTEGER,
		stdout TEXT,
		stderr TEXT,
		logs TEXT,
		metadata_preserved BOOLEAN DEFAULT 0,
		metadata_summary TEXT,
		job_params TEXT,
		FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
		FOREIGN KEY (file_index_id) REFERENCES files_index(id) ON DELETE SET NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_workflow_run_workflow ON workflow_runs(workflow_id);
	CREATE INDEX IF NOT EXISTS idx_workflow_run_status ON workflow_runs(status);
	CREATE INDEX IF NOT EXISTS idx_workflow_run_start ON workflow_runs(start_time DESC);
	
	CREATE TABLE IF NOT EXISTS workflows_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workflow_id INTEGER NOT NULL,
		yaml TEXT NOT NULL,
		edited_by TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
	);
	
	CREATE INDEX IF NOT EXISTS idx_workflow_version ON workflows_versions(workflow_id, created_at DESC);
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
	query := `INSERT INTO tasks_history (file_path, converter_name, status, error_message, console_output, duration_ms)
	          VALUES (?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query,
		task.FilePath,
		task.ConverterName,
		task.Status,
		task.ErrorMessage,
		task.ConsoleOutput,
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
	query := `SELECT id, file_path, converter_name, status, error_message, duration_ms, created_at, console_output
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
			&task.ConsoleOutput,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// GetTaskByID retrieves a single task by ID
func (db *DB) GetTaskByID(id int64) (*TaskHistory, error) {
	query := `SELECT id, file_path, converter_name, status, error_message, duration_ms, created_at, console_output
	          FROM tasks_history
	          WHERE id = ?`

	task := &TaskHistory{}
	err := db.conn.QueryRow(query, id).Scan(
		&task.ID,
		&task.FilePath,
		&task.ConverterName,
		&task.Status,
		&task.ErrorMessage,
		&task.DurationMs,
		&task.CreatedAt,
		&task.ConsoleOutput,
	)
	if err != nil {
		return nil, err
	}

	return task, nil
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

// Workflow operations

// CreateWorkflow inserts a new workflow
func (db *DB) CreateWorkflow(wf *Workflow) error {
	query := `INSERT INTO workflows (name, description, yaml, enabled, created_by, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	result, err := db.conn.Exec(query, wf.Name, wf.Description, wf.YAML, wf.Enabled, wf.CreatedBy, now, now)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	wf.ID = id
	wf.CreatedAt = now
	wf.UpdatedAt = now

	// Create initial version
	return db.CreateWorkflowVersion(id, wf.YAML, wf.CreatedBy)
}

// UpdateWorkflow updates an existing workflow
func (db *DB) UpdateWorkflow(wf *Workflow) error {
	query := `UPDATE workflows SET name = ?, description = ?, yaml = ?, enabled = ?, updated_at = ?
	          WHERE id = ?`

	now := time.Now()
	_, err := db.conn.Exec(query, wf.Name, wf.Description, wf.YAML, wf.Enabled, now, wf.ID)
	if err != nil {
		return err
	}

	// Create version snapshot
	return db.CreateWorkflowVersion(wf.ID, wf.YAML, wf.CreatedBy)
}

// GetWorkflow retrieves a workflow by ID
func (db *DB) GetWorkflow(id int64) (*Workflow, error) {
	query := `SELECT id, name, description, yaml, enabled, created_by, created_at, updated_at
	          FROM workflows WHERE id = ?`

	wf := &Workflow{}
	err := db.conn.QueryRow(query, id).Scan(
		&wf.ID, &wf.Name, &wf.Description, &wf.YAML, &wf.Enabled,
		&wf.CreatedBy, &wf.CreatedAt, &wf.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return wf, nil
}

// GetWorkflowByName retrieves a workflow by name
func (db *DB) GetWorkflowByName(name string) (*Workflow, error) {
	query := `SELECT id, name, description, yaml, enabled, created_by, created_at, updated_at
	          FROM workflows WHERE name = ?`

	wf := &Workflow{}
	err := db.conn.QueryRow(query, name).Scan(
		&wf.ID, &wf.Name, &wf.Description, &wf.YAML, &wf.Enabled,
		&wf.CreatedBy, &wf.CreatedAt, &wf.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return wf, nil
}

// ListWorkflows lists all workflows with pagination
func (db *DB) ListWorkflows(limit, offset int) ([]*Workflow, error) {
	query := `SELECT id, name, description, yaml, enabled, created_by, created_at, updated_at
	          FROM workflows
	          ORDER BY created_at DESC
	          LIMIT ? OFFSET ?`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workflows := []*Workflow{}
	for rows.Next() {
		wf := &Workflow{}
		err := rows.Scan(
			&wf.ID, &wf.Name, &wf.Description, &wf.YAML, &wf.Enabled,
			&wf.CreatedBy, &wf.CreatedAt, &wf.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}

	return workflows, rows.Err()
}

// DeleteWorkflow deletes a workflow
func (db *DB) DeleteWorkflow(id int64) error {
	query := `DELETE FROM workflows WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	return err
}

// CreateWorkflowVersion creates a version snapshot
func (db *DB) CreateWorkflowVersion(workflowID int64, yaml, editedBy string) error {
	query := `INSERT INTO workflows_versions (workflow_id, yaml, edited_by, created_at)
	          VALUES (?, ?, ?, ?)`
	_, err := db.conn.Exec(query, workflowID, yaml, editedBy, time.Now())
	return err
}

// ListWorkflowVersions lists versions for a workflow
func (db *DB) ListWorkflowVersions(workflowID int64, limit int) ([]*WorkflowVersion, error) {
	query := `SELECT id, workflow_id, yaml, edited_by, created_at
	          FROM workflows_versions
	          WHERE workflow_id = ?
	          ORDER BY created_at DESC
	          LIMIT ?`

	rows, err := db.conn.Query(query, workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := []*WorkflowVersion{}
	for rows.Next() {
		v := &WorkflowVersion{}
		err := rows.Scan(&v.ID, &v.WorkflowID, &v.YAML, &v.EditedBy, &v.CreatedAt)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// CreateWorkflowRun creates a new workflow run
func (db *DB) CreateWorkflowRun(run *WorkflowRun) error {
	query := `INSERT INTO workflow_runs (workflow_id, workflow_name, file_index_id, file_path,
	          status, start_time, job_params)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := db.conn.Exec(query, run.WorkflowID, run.WorkflowName, run.FileIndexID,
		run.FilePath, run.Status, run.StartTime, run.JobParams)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	run.ID = id
	return nil
}

// UpdateWorkflowRun updates a workflow run
func (db *DB) UpdateWorkflowRun(run *WorkflowRun) error {
	query := `UPDATE workflow_runs SET status = ?, end_time = ?, duration_ms = ?,
	          exit_code = ?, stdout = ?, stderr = ?, logs = ?,
	          metadata_preserved = ?, metadata_summary = ?
	          WHERE id = ?`

	_, err := db.conn.Exec(query, run.Status, run.EndTime, run.DurationMs,
		run.ExitCode, run.Stdout, run.Stderr, run.Logs,
		run.MetadataPreserved, run.MetadataSummary, run.ID)
	return err
}

// GetWorkflowRun retrieves a workflow run by ID
func (db *DB) GetWorkflowRun(id int64) (*WorkflowRun, error) {
	query := `SELECT id, workflow_id, workflow_name, file_index_id, file_path, status,
	          start_time, end_time, duration_ms, exit_code, stdout, stderr, logs,
	          metadata_preserved, metadata_summary, job_params
	          FROM workflow_runs WHERE id = ?`

	run := &WorkflowRun{}
	var endTime sql.NullTime
	var exitCode sql.NullInt64
	var fileIndexID sql.NullInt64

	err := db.conn.QueryRow(query, id).Scan(
		&run.ID, &run.WorkflowID, &run.WorkflowName, &fileIndexID, &run.FilePath,
		&run.Status, &run.StartTime, &endTime, &run.DurationMs, &exitCode,
		&run.Stdout, &run.Stderr, &run.Logs, &run.MetadataPreserved,
		&run.MetadataSummary, &run.JobParams,
	)
	if err != nil {
		return nil, err
	}

	if endTime.Valid {
		run.EndTime = &endTime.Time
	}
	if exitCode.Valid {
		code := int(exitCode.Int64)
		run.ExitCode = &code
	}
	if fileIndexID.Valid {
		fid := fileIndexID.Int64
		run.FileIndexID = &fid
	}

	return run, nil
}

// ListWorkflowRuns lists runs for a workflow
func (db *DB) ListWorkflowRuns(workflowID int64, limit, offset int) ([]*WorkflowRun, error) {
	query := `SELECT id, workflow_id, workflow_name, file_index_id, file_path, status,
	          start_time, end_time, duration_ms, exit_code, stdout, stderr, logs,
	          metadata_preserved, metadata_summary, job_params
	          FROM workflow_runs
	          WHERE workflow_id = ?
	          ORDER BY start_time DESC
	          LIMIT ? OFFSET ?`

	rows, err := db.conn.Query(query, workflowID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanWorkflowRuns(rows)
}

// ListAllWorkflowRuns lists all runs with pagination
func (db *DB) ListAllWorkflowRuns(limit, offset int) ([]*WorkflowRun, error) {
	query := `SELECT id, workflow_id, workflow_name, file_index_id, file_path, status,
	          start_time, end_time, duration_ms, exit_code, stdout, stderr, logs,
	          metadata_preserved, metadata_summary, job_params
	          FROM workflow_runs
	          ORDER BY start_time DESC
	          LIMIT ? OFFSET ?`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanWorkflowRuns(rows)
}

func (db *DB) scanWorkflowRuns(rows *sql.Rows) ([]*WorkflowRun, error) {
	runs := []*WorkflowRun{}
	for rows.Next() {
		run := &WorkflowRun{}
		var endTime sql.NullTime
		var exitCode sql.NullInt64
		var fileIndexID sql.NullInt64

		err := rows.Scan(
			&run.ID, &run.WorkflowID, &run.WorkflowName, &fileIndexID, &run.FilePath,
			&run.Status, &run.StartTime, &endTime, &run.DurationMs, &exitCode,
			&run.Stdout, &run.Stderr, &run.Logs, &run.MetadataPreserved,
			&run.MetadataSummary, &run.JobParams,
		)
		if err != nil {
			return nil, err
		}

		if endTime.Valid {
			run.EndTime = &endTime.Time
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			run.ExitCode = &code
		}
		if fileIndexID.Valid {
			fid := fileIndexID.Int64
			run.FileIndexID = &fid
		}

		runs = append(runs, run)
	}

	return runs, rows.Err()
}

// ClearIndex clears all file index entries
func (db *DB) ClearIndex() error {
	_, err := db.conn.Exec(`DELETE FROM files_index`)
	return err
}
