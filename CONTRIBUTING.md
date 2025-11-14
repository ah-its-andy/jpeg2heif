# Contributing to JPEG2HEIF

Thank you for your interest in contributing to JPEG2HEIF! This document provides guidelines for adding new format converters.

## Adding a New Converter

The JPEG2HEIF project uses a pluggable converter architecture that makes it easy to add support for new format conversions.

### Step 1: Create Converter File

Create a new file in `internal/converter/` for your converter. For example, to add PNG to HEIC conversion:

```bash
touch internal/converter/png2heic.go
```

### Step 2: Implement the Converter Interface

Your converter must implement the `Converter` interface:

```go
package converter

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

// PNG2HEICConverter converts PNG files to HEIC format
type PNG2HEICConverter struct{}

func init() {
    // Register the converter on package initialization
    Register(&PNG2HEICConverter{})
}

func (c *PNG2HEICConverter) Name() string {
    return "png2heic"
}

func (c *PNG2HEICConverter) CanConvert(srcPath string, srcMime string) bool {
    ext := strings.ToLower(filepath.Ext(srcPath))
    return ext == ".png" || strings.Contains(srcMime, "png")
}

func (c *PNG2HEICConverter) TargetFormat() string {
    return "heic"
}

func (c *PNG2HEICConverter) Convert(ctx context.Context, srcPath string, dstPath string, opts ConvertOptions) (MetaResult, error) {
    result := MetaResult{
        MetadataPreserved: false,
        ConversionLog:     "",
    }

    // 1. Check if external tools are available
    if _, err := exec.LookPath("convert"); err != nil {
        return result, fmt.Errorf("ImageMagick 'convert' not found")
    }

    // 2. Create temporary output file
    tmpDir := opts.TempDir
    if tmpDir == "" {
        tmpDir = os.TempDir()
    }
    tmpFile := filepath.Join(tmpDir, fmt.Sprintf("png2heif_%d.heic", time.Now().UnixNano()))
    defer os.Remove(tmpFile)

    // 3. Perform conversion using external tool
    quality := opts.Quality
    if quality <= 0 || quality > 100 {
        quality = 85
    }

    cmd := exec.CommandContext(ctx, "convert", srcPath, "-quality", fmt.Sprintf("%d", quality), tmpFile)
    output, err := cmd.CombinedOutput()
    result.ConversionLog += fmt.Sprintf("convert output:\n%s\n", string(output))
    
    if err != nil {
        return result, fmt.Errorf("conversion failed: %w, output: %s", err, string(output))
    }

    // 4. Verify temporary file was created
    if _, err := os.Stat(tmpFile); err != nil {
        return result, fmt.Errorf("conversion did not create output file: %w", err)
    }

    // 5. Handle metadata preservation
    if opts.PreserveMetadata {
        // Use exiftool or similar to preserve metadata
        // ... metadata preservation logic ...
    }

    // 6. Create destination directory
    dstDir := filepath.Dir(dstPath)
    if err := os.MkdirAll(dstDir, 0755); err != nil {
        return result, fmt.Errorf("failed to create destination directory: %w", err)
    }

    // 7. Atomic rename to final destination
    if err := os.Rename(tmpFile, dstPath); err != nil {
        return result, fmt.Errorf("failed to move output to destination: %w", err)
    }

    result.ConversionLog += fmt.Sprintf("Conversion completed: %s -> %s\n", srcPath, dstPath)
    return result, nil
}
```

### Step 3: Add Tests

Add tests for your converter in `tests/converter_test.go`:

```go
func TestPNG2HEICConverter(t *testing.T) {
    conv, ok := converter.Get("png2heic")
    if !ok {
        t.Fatal("png2heic converter not found")
    }

    if !conv.CanConvert("test.png", "") {
        t.Error("Should be able to convert PNG files")
    }

    if conv.TargetFormat() != "heic" {
        t.Errorf("Expected target format 'heic', got '%s'", conv.TargetFormat())
    }
}
```

### Step 4: Update Documentation

1. Update `README.md` to list the new converter in the "Built-in Converters" section
2. Document any external tool requirements in the "External Dependencies" section
3. Add examples of supported file types

### Step 5: Test Your Converter

```bash
# Run unit tests
make test-short

# Run integration tests (requires external tools)
make test

# Build and test manually
make build
./jpeg2heif
```

## Converter Best Practices

### 1. Error Handling

Always provide detailed error messages with context:

```go
if err != nil {
    return result, fmt.Errorf("failed to convert %s: %w", srcPath, err)
}
```

### 2. Logging

Log important steps to `result.ConversionLog`:

```go
result.ConversionLog += fmt.Sprintf("Step completed successfully\n")
```

### 3. Cleanup

Always clean up temporary files:

```go
tmpFile := "/path/to/temp"
defer os.Remove(tmpFile)
```

### 4. Timeouts

Respect the context timeout:

```go
cmd := exec.CommandContext(ctx, "tool", args...)
```

### 5. Metadata Preservation

Document which metadata fields are preserved in `MetaResult`:

```go
result.MetadataPreserved = true
result.MetadataSummary = "EXIF, XMP, and GPS data preserved"
```

## Testing Requirements

Your converter must:

1. ✅ Pass all unit tests
2. ✅ Handle missing external tools gracefully
3. ✅ Validate input files
4. ✅ Clean up temporary files on error
5. ✅ Respect conversion quality settings
6. ✅ Support metadata preservation when enabled
7. ✅ Return detailed error messages

## External Tool Requirements

If your converter requires external tools:

1. Document installation instructions in README
2. Check for tool availability in `Convert()`
3. Provide clear error messages when tools are missing
4. Include tool version requirements

Example for Dockerfile:

```dockerfile
RUN apt-get update && apt-get install -y \
    your-tool-package=version \
    && rm -rf /var/lib/apt/lists/*
```

## Submission Checklist

Before submitting a pull request:

- [ ] Converter implements all interface methods
- [ ] Registered in `init()` function
- [ ] Unit tests added and passing
- [ ] Integration tests added (if applicable)
- [ ] README.md updated
- [ ] External dependencies documented
- [ ] Code formatted with `go fmt`
- [ ] All existing tests still pass

## Example Converters

See existing converters for reference:

- `internal/converter/jpeg2heic.go` - Complete example with metadata preservation
- Future converters will be added to this list

## Questions?

Open an issue on GitHub if you have questions about adding a converter.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
