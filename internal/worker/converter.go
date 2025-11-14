package worker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/utils"
	"github.com/rwcarlsen/goexif/exif"
	"gorm.io/gorm"
)

// Converter performs JPEG->HEIC conversion via system tools (ImageMagick + exiftool)

type Converter struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewConverter(cfg *config.Config, dbConn *gorm.DB) *Converter {
	return &Converter{cfg: cfg, db: dbConn}
}

func (c *Converter) Convert(ctx context.Context, rec *db.FileIndex) (metadataPreserved bool, metadataSummary string, outLog string, err error) {
	start := time.Now()
	var logBuf bytes.Buffer
	defer func() {
		logBuf.WriteString(fmt.Sprintf("duration=%s\n", time.Since(start)))
		outLog = logBuf.String()
	}()

	src := rec.FilePath
	if !utils.IsJPEG(src) {
		return false, "not jpeg", logBuf.String(), fmt.Errorf("not a jpeg: %s", src)
	}

	// Ensure stability before conversion
	if err := utils.WaitFileStable(src, time.Duration(c.cfg.MetadataStabilityDelay)*time.Second); err != nil {
		return false, "file not stable", logBuf.String(), fmt.Errorf("file not stable: %w", err)
	}

	// Read source DateTimeOriginal
	var dtoStr string
	func() {
		f, err := os.Open(src)
		if err != nil {
			logBuf.WriteString("open src for exif failed: " + err.Error() + "\n")
			return
		}
		defer f.Close()
		x, err := exif.Decode(f)
		if err != nil {
			logBuf.WriteString("exif decode failed: " + err.Error() + "\n")
			return
		}
		if tm, err := x.DateTime(); err == nil {
			dtoStr = tm.Format("2006:01:02 15:04:05")
		}
	}()

	outPath := utils.TargetHEICPath(src)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return false, "mkdir out dir", logBuf.String(), err
	}
	tmpPath := outPath + ".tmp"
	_ = os.Remove(tmpPath)

	// Convert via ImageMagick (magick)
	quality := fmt.Sprintf("%d", c.cfg.ConvertQuality)
	cmd := exec.CommandContext(ctx, "magick", src, "-quality", quality, tmpPath)
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf
	if err := cmd.Run(); err != nil {
		return false, "convert failed", logBuf.String(), fmt.Errorf("magick failed: %w", err)
	}

	// Copy metadata via exiftool if requested
	if c.cfg.PreserveMetadata {
		exifCmd := exec.CommandContext(ctx, "exiftool", "-overwrite_original", "-TagsFromFile", src, "-all:all", tmpPath)
		exifCmd.Stdout = &logBuf
		exifCmd.Stderr = &logBuf
		if err := exifCmd.Run(); err != nil {
			logBuf.WriteString("exiftool failed: " + err.Error() + "\n")
		}
	}

	// Atomic rename
	finalPath := outPath
	if _, err := os.Stat(finalPath); err == nil {
		// Exists; rename with timestamp to avoid overwrite
		ts := time.Now().Format("20060102T150405")
		base := strings.TrimSuffix(filepath.Base(finalPath), filepath.Ext(finalPath))
		finalPath = filepath.Join(filepath.Dir(finalPath), fmt.Sprintf("%s_%s.heic", base, ts))
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return false, "rename failed", logBuf.String(), err
	}

	// Verify DateTimeOriginal via exiftool if available
	preserved := false
	summary := []string{}
	if c.cfg.PreserveMetadata {
		var out bytes.Buffer
		verify := exec.CommandContext(ctx, "exiftool", "-DateTimeOriginal", "-s3", finalPath)
		verify.Stdout = &out
		if err := verify.Run(); err == nil {
			val := strings.TrimSpace(out.String())
			if val != "" && dtoStr != "" && val == dtoStr {
				preserved = true
				summary = append(summary, "DateTimeOriginal preserved")
			} else if val != "" {
				summary = append(summary, "DateTimeOriginal written but differs")
			} else {
				summary = append(summary, "DateTimeOriginal not found in HEIC")
			}
		} else {
			summary = append(summary, "exiftool verify unavailable")
		}
	} else {
		summary = append(summary, "metadata preservation disabled")
	}

	return preserved, strings.Join(summary, "; "), logBuf.String(), nil
}
