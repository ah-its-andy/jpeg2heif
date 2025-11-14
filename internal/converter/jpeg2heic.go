package converter

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// JPEG2HEICConverter converts JPEG files to HEIC format
type JPEG2HEICConverter struct{}

// NewJPEG2HEICConverter creates a new JPEG2HEIC converter
func NewJPEG2HEICConverter() *JPEG2HEICConverter {
	return &JPEG2HEICConverter{}
}

func (c *JPEG2HEICConverter) Name() string {
	return "jpeg2heic"
}

func (c *JPEG2HEICConverter) CanConvert(srcPath string, srcMime string) bool {
	ext := strings.ToLower(filepath.Ext(srcPath))
	return ext == ".jpg" || ext == ".jpeg" || strings.Contains(srcMime, "jpeg")
}

func (c *JPEG2HEICConverter) TargetFormat() string {
	return "heic"
}

func (c *JPEG2HEICConverter) Convert(ctx context.Context, srcPath string, dstPath string, opts ConvertOptions) (MetaResult, error) {
	result := MetaResult{
		MetadataPreserved: false,
		ConversionLog:     "",
	}

	// Check if external tools are available
	if err := checkExternalTools(); err != nil {
		return result, fmt.Errorf("external tools check failed: %w", err)
	}

	// Extract source metadata first
	sourceMeta, err := extractMetadata(srcPath)
	if err != nil {
		result.ConversionLog += fmt.Sprintf("Warning: failed to extract source metadata: %v\n", err)
	}

	// Create temporary output file
	tmpDir := opts.TempDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("jpeg2heif_%d.heic", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	// Convert JPEG to HEIC using heif-enc
	quality := opts.Quality
	if quality <= 0 || quality > 100 {
		quality = 85
	}

	encCmd := exec.CommandContext(ctx, "heif-enc", "-q", fmt.Sprintf("%d", quality), "-o", tmpFile, srcPath)
	output, err := encCmd.CombinedOutput()
	result.ConversionLog += fmt.Sprintf("heif-enc output:\n%s\n", string(output))

	if err != nil {
		return result, fmt.Errorf("heif-enc failed: %w, output: %s", err, string(output))
	}

	// Verify temporary file was created
	if _, err := os.Stat(tmpFile); err != nil {
		return result, fmt.Errorf("heif-enc did not create output file: %w", err)
	}

	// Inject metadata if preservation is enabled
	if opts.PreserveMetadata {
		if err := injectMetadata(srcPath, tmpFile); err != nil {
			result.ConversionLog += fmt.Sprintf("Warning: metadata injection failed: %v\n", err)
		} else {
			result.MetadataPreserved = true
			result.MetadataSummary = "Full EXIF/XMP metadata preserved"
		}
	} else {
		// At minimum, preserve DateTimeOriginal
		if err := preserveDateTimeOriginal(srcPath, tmpFile, sourceMeta); err != nil {
			result.ConversionLog += fmt.Sprintf("Warning: DateTimeOriginal preservation failed: %v\n", err)
		} else {
			result.MetadataPreserved = true
			result.MetadataSummary = "DateTimeOriginal preserved"
		}
	}

	// Verify DateTimeOriginal was preserved
	targetMeta, err := extractMetadata(tmpFile)
	if err == nil && sourceMeta != nil && targetMeta != nil {
		srcTime := sourceMeta["DateTimeOriginal"]
		dstTime := targetMeta["DateTimeOriginal"]
		if srcTime != "" && srcTime == dstTime {
			result.ConversionLog += fmt.Sprintf("Verified: DateTimeOriginal preserved (%s)\n", srcTime)
		} else {
			result.ConversionLog += fmt.Sprintf("Warning: DateTimeOriginal mismatch (src: %s, dst: %s)\n", srcTime, dstTime)
		}
	}

	// Create destination directory if needed
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy file to destination (handles cross-device moves)
	if err := copyFile(tmpFile, dstPath); err != nil {
		return result, fmt.Errorf("failed to copy output to destination: %w", err)
	}

	result.ConversionLog += fmt.Sprintf("Conversion completed successfully: %s -> %s\n", srcPath, dstPath)
	return result, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Sync to ensure data is written to disk
	return destFile.Sync()
}

// checkExternalTools verifies that required external tools are available
func checkExternalTools() error {
	tools := []string{"heif-enc", "exiftool"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool not found: %s", tool)
		}
	}
	return nil
}

// extractMetadata extracts EXIF metadata from a file
func extractMetadata(filePath string) (map[string]string, error) {
	metadata := make(map[string]string)

	// Try using exiftool first (more reliable)
	cmd := exec.Command("exiftool", "-s", "-s", "-s", "-DateTimeOriginal", "-CreateDate", "-ModifyDate", filePath)
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 && lines[0] != "" {
			metadata["DateTimeOriginal"] = strings.TrimSpace(lines[0])
		}
	}

	// Also try using goexif as backup
	f, err := os.Open(filePath)
	if err != nil {
		return metadata, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return metadata, err
	}

	// Get DateTimeOriginal
	if dt, err := x.DateTime(); err == nil {
		metadata["DateTimeOriginal"] = dt.Format("2006:01:02 15:04:05")
	}

	return metadata, nil
}

// injectMetadata copies all metadata from source to destination
func injectMetadata(srcPath, dstPath string) error {
	cmd := exec.Command("exiftool", "-TagsFromFile", srcPath, "-all:all", "-overwrite_original", dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w, output: %s", err, string(output))
	}
	return nil
}

// preserveDateTimeOriginal preserves only the DateTimeOriginal tag
func preserveDateTimeOriginal(srcPath, dstPath string, sourceMeta map[string]string) error {
	var dateTime string

	// Try to get from extracted metadata
	if sourceMeta != nil {
		dateTime = sourceMeta["DateTimeOriginal"]
	}

	// If not found, try extracting again
	if dateTime == "" {
		meta, err := extractMetadata(srcPath)
		if err != nil || meta["DateTimeOriginal"] == "" {
			return fmt.Errorf("could not find DateTimeOriginal in source file")
		}
		dateTime = meta["DateTimeOriginal"]
	}

	// Inject DateTimeOriginal into destination
	cmd := exec.Command("exiftool",
		fmt.Sprintf("-DateTimeOriginal=%s", dateTime),
		"-overwrite_original",
		dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w, output: %s", err, string(output))
	}

	return nil
}
