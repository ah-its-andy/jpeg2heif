package converter

import (
	"context"
	"time"
)

// ConvertOptions holds configuration for a conversion operation
type ConvertOptions struct {
	Quality          int               // Quality setting (1-100)
	PreserveMetadata bool              // Whether to preserve all metadata
	TempDir          string            // Temporary directory for intermediate files
	Timeout          time.Duration     // Conversion timeout
	ExtraArgs        map[string]string // Converter-specific arguments
}

// MetaResult contains metadata preservation results
type MetaResult struct {
	MetadataPreserved bool   // Whether metadata was successfully preserved
	MetadataSummary   string // Human-readable summary of preserved metadata
	SourceMD5         string // MD5 of source file
	OutputMD5         string // MD5 of output file
	ConversionLog     string // Detailed conversion log
}

// Converter defines the interface for format converters
type Converter interface {
	// Name returns the unique name of this converter
	Name() string

	// CanConvert checks if this converter can handle the given source file
	CanConvert(srcPath string, srcMime string) bool

	// TargetFormat returns the file extension of the output format (without dot)
	TargetFormat() string

	// Convert performs the conversion from srcPath to dstPath
	// It should:
	// 1. Create temporary output file
	// 2. Perform format conversion
	// 3. Extract and inject metadata
	// 4. Verify critical metadata (e.g., DateTimeOriginal)
	// 5. Atomically move temp file to dstPath
	Convert(ctx context.Context, srcPath string, dstPath string, opts ConvertOptions) (MetaResult, error)
}

// ConverterInfo provides information about a registered converter
type ConverterInfo struct {
	Name               string   `json:"name"`
	TargetFormat       string   `json:"target_format"`
	SupportedMimeTypes []string `json:"supported_mime_types"`
	Enabled            bool     `json:"enabled"`
}
