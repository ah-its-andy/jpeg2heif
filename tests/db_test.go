package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/util"
)

func TestDatabaseInit(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize database
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file was not created")
	}
}

func TestFileIndexOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Test insert
	file := &db.FileIndex{
		FilePath:          "/test/path/file.jpg",
		FileMD5:           "abc123",
		Status:            "pending",
		ConverterName:     "jpeg2heic",
		MetadataPreserved: false,
		MetadataSummary:   "",
	}

	err = database.UpsertFileIndex(file)
	if err != nil {
		t.Fatalf("Failed to insert file index: %v", err)
	}

	if file.ID == 0 {
		t.Error("File ID should be set after insert")
	}

	// Test retrieve
	retrieved, err := database.GetFileIndex(file.FilePath)
	if err != nil {
		t.Fatalf("Failed to retrieve file index: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved file should not be nil")
	}

	if retrieved.FilePath != file.FilePath {
		t.Errorf("Expected file path %s, got %s", file.FilePath, retrieved.FilePath)
	}

	if retrieved.FileMD5 != file.FileMD5 {
		t.Errorf("Expected MD5 %s, got %s", file.FileMD5, retrieved.FileMD5)
	}

	// Test update
	file.Status = "success"
	file.MetadataPreserved = true
	file.MetadataSummary = "DateTimeOriginal preserved"

	err = database.UpsertFileIndex(file)
	if err != nil {
		t.Fatalf("Failed to update file index: %v", err)
	}

	retrieved, err = database.GetFileIndex(file.FilePath)
	if err != nil {
		t.Fatalf("Failed to retrieve updated file index: %v", err)
	}

	if retrieved.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", retrieved.Status)
	}

	if !retrieved.MetadataPreserved {
		t.Error("MetadataPreserved should be true")
	}
}

func TestListFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Insert multiple files
	files := []*db.FileIndex{
		{FilePath: "/test/file1.jpg", FileMD5: "md51", Status: "success", ConverterName: "jpeg2heic"},
		{FilePath: "/test/file2.jpg", FileMD5: "md52", Status: "failed", ConverterName: "jpeg2heic"},
		{FilePath: "/test/file3.jpg", FileMD5: "md53", Status: "pending", ConverterName: "jpeg2heic"},
	}

	for _, file := range files {
		if err := database.UpsertFileIndex(file); err != nil {
			t.Fatalf("Failed to insert file: %v", err)
		}
	}

	// Test list all
	allFiles, err := database.ListFiles("", 10, 0)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(allFiles) != 3 {
		t.Errorf("Expected 3 files, got %d", len(allFiles))
	}

	// Test list by status
	successFiles, err := database.ListFiles("success", 10, 0)
	if err != nil {
		t.Fatalf("Failed to list success files: %v", err)
	}

	if len(successFiles) != 1 {
		t.Errorf("Expected 1 success file, got %d", len(successFiles))
	}

	if successFiles[0].Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", successFiles[0].Status)
	}
}

func TestTaskHistory(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Insert task
	task := &db.TaskHistory{
		FilePath:      "/test/file.jpg",
		ConverterName: "jpeg2heic",
		Status:        "success",
		ErrorMessage:  "",
		DurationMs:    1500,
	}

	err = database.InsertTaskHistory(task)
	if err != nil {
		t.Fatalf("Failed to insert task history: %v", err)
	}

	if task.ID == 0 {
		t.Error("Task ID should be set after insert")
	}

	// List tasks
	tasks, err := database.ListTasks(10, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	if tasks[0].FilePath != task.FilePath {
		t.Errorf("Expected file path %s, got %s", task.FilePath, tasks[0].FilePath)
	}
}

func TestStats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Insert files with different statuses
	files := []*db.FileIndex{
		{FilePath: "/test/file1.jpg", FileMD5: "md51", Status: "success", ConverterName: "jpeg2heic"},
		{FilePath: "/test/file2.jpg", FileMD5: "md52", Status: "success", ConverterName: "jpeg2heic"},
		{FilePath: "/test/file3.jpg", FileMD5: "md53", Status: "failed", ConverterName: "jpeg2heic"},
		{FilePath: "/test/file4.jpg", FileMD5: "md54", Status: "pending", ConverterName: "jpeg2heic"},
	}

	for _, file := range files {
		if err := database.UpsertFileIndex(file); err != nil {
			t.Fatalf("Failed to insert file: %v", err)
		}
	}

	// Get stats
	stats, err := database.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalFiles != 4 {
		t.Errorf("Expected 4 total files, got %d", stats.TotalFiles)
	}

	if stats.SuccessCount != 2 {
		t.Errorf("Expected 2 success files, got %d", stats.SuccessCount)
	}

	if stats.FailedCount != 1 {
		t.Errorf("Expected 1 failed file, got %d", stats.FailedCount)
	}

	if stats.PendingCount != 1 {
		t.Errorf("Expected 1 pending file, got %d", stats.PendingCount)
	}
}

func TestMD5Calculation(t *testing.T) {
	// Create a test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate MD5
	md5Hash, err := util.CalculateMD5(testFile, 8192)
	if err != nil {
		t.Fatalf("Failed to calculate MD5: %v", err)
	}

	// Expected MD5 for "Hello, World!"
	expected := "65a8e27d8879283831b664bd8b7f0ad4"
	if md5Hash != expected {
		t.Errorf("Expected MD5 %s, got %s", expected, md5Hash)
	}
}

func TestMD5ChunkSize(t *testing.T) {
	// Create a larger test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.bin")
	content := make([]byte, 1024*10) // 10KB
	for i := range content {
		content[i] = byte(i % 256)
	}

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate MD5 with different chunk sizes
	md5_1, err := util.CalculateMD5(testFile, 1024)
	if err != nil {
		t.Fatalf("Failed to calculate MD5 with chunk size 1024: %v", err)
	}

	md5_2, err := util.CalculateMD5(testFile, 4096)
	if err != nil {
		t.Fatalf("Failed to calculate MD5 with chunk size 4096: %v", err)
	}

	// Results should be identical regardless of chunk size
	if md5_1 != md5_2 {
		t.Errorf("MD5 hashes don't match: %s != %s", md5_1, md5_2)
	}
}
