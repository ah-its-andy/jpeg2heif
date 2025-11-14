package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/converter"
)

func init() {
	// Register converters for testing
	converter.Register(converter.NewJPEG2HEICConverter())
}

func TestConverterRegistry(t *testing.T) {
	// Test that converters are registered
	converters := converter.List()
	if len(converters) == 0 {
		t.Fatal("No converters registered")
	}

	// Test that jpeg2heic converter exists
	jpeg2heic, ok := converter.Get("jpeg2heic")
	if !ok {
		t.Fatal("jpeg2heic converter not found")
	}

	if jpeg2heic.Name() != "jpeg2heic" {
		t.Errorf("Expected converter name 'jpeg2heic', got '%s'", jpeg2heic.Name())
	}

	if jpeg2heic.TargetFormat() != "heic" {
		t.Errorf("Expected target format 'heic', got '%s'", jpeg2heic.TargetFormat())
	}
}

func TestConverterCanConvert(t *testing.T) {
	jpeg2heic, ok := converter.Get("jpeg2heic")
	if !ok {
		t.Fatal("jpeg2heic converter not found")
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"test.jpg", true},
		{"test.jpeg", true},
		{"test.JPG", true},
		{"test.JPEG", true},
		{"test.png", false},
		{"test.gif", false},
		{"test.heic", false},
	}

	for _, tt := range tests {
		result := jpeg2heic.CanConvert(tt.path, "")
		if result != tt.expected {
			t.Errorf("CanConvert(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestConverterFindConverter(t *testing.T) {
	tests := []struct {
		path         string
		shouldFind   bool
		expectedName string
	}{
		{"test.jpg", true, "jpeg2heic"},
		{"test.jpeg", true, "jpeg2heic"},
		{"test.txt", false, ""},
	}

	for _, tt := range tests {
		conv, err := converter.FindConverter(tt.path, "")
		if tt.shouldFind {
			if err != nil {
				t.Errorf("FindConverter(%s) unexpected error: %v", tt.path, err)
			}
			if conv.Name() != tt.expectedName {
				t.Errorf("FindConverter(%s) = %s, expected %s", tt.path, conv.Name(), tt.expectedName)
			}
		} else {
			if err == nil {
				t.Errorf("FindConverter(%s) should have returned error", tt.path)
			}
		}
	}
}

func TestConverterEnableDisable(t *testing.T) {
	converterName := "jpeg2heic"

	// Initially should be enabled
	if !converter.IsEnabled(converterName) {
		t.Error("Converter should be enabled by default")
	}

	// Disable it
	if err := converter.Disable(converterName); err != nil {
		t.Fatalf("Failed to disable converter: %v", err)
	}

	if converter.IsEnabled(converterName) {
		t.Error("Converter should be disabled")
	}

	// Enable it again
	if err := converter.Enable(converterName); err != nil {
		t.Fatalf("Failed to enable converter: %v", err)
	}

	if !converter.IsEnabled(converterName) {
		t.Error("Converter should be enabled")
	}
}

// Integration test - requires external tools
func TestJPEG2HEICConversion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if external tools are available
	if !checkExternalToolsAvailable() {
		t.Skip("External tools (heif-enc, exiftool) not available")
	}

	// Create a test JPEG file (1x1 pixel red square)
	testDir := t.TempDir()
	srcPath := filepath.Join(testDir, "test.jpg")
	dstPath := filepath.Join(testDir, "test.heic")

	// Create a minimal valid JPEG
	jpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x14, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x03, 0xFF, 0xC4, 0x00, 0x14, 0x10, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00,
		0x37, 0xFF, 0xD9,
	}

	if err := os.WriteFile(srcPath, jpegData, 0644); err != nil {
		t.Fatalf("Failed to create test JPEG: %v", err)
	}

	// Get converter
	conv, ok := converter.Get("jpeg2heic")
	if !ok {
		t.Fatal("Failed to get converter")
	}

	// Perform conversion
	opts := converter.ConvertOptions{
		Quality:          85,
		PreserveMetadata: true,
		TempDir:          testDir,
		Timeout:          30 * time.Second,
	}

	ctx := context.Background()
	result, err := conv.Convert(ctx, srcPath, dstPath, opts)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Verify output file exists
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}

	// Verify result
	t.Logf("Conversion result: %+v", result)
	t.Logf("Conversion log: %s", result.ConversionLog)
}

func checkExternalToolsAvailable() bool {
	// Try to run heif-enc and exiftool
	tools := []string{"heif-enc", "exiftool"}
	for _, tool := range tools {
		if _, err := os.Stat("/usr/bin/" + tool); os.IsNotExist(err) {
			if _, err := os.Stat("/usr/local/bin/" + tool); os.IsNotExist(err) {
				return false
			}
		}
	}
	return true
}
