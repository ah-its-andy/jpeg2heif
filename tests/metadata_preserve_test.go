package tests

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/utils"
	"github.com/ah-its-andy/jpeg2heif/internal/worker"
)

func TestMetadataPreserved_DateTimeOriginal(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("magick not found")
	}
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not found")
	}

	d := t.TempDir()
	imgDir := filepath.Join(d, "a", "b", "c")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(imgDir, "meta.jpg")
	createJPEG(t, src)

	// write DateTimeOriginal via exiftool
	dto := time.Date(2023, 11, 14, 10, 30, 0, 0, time.UTC).Format("2006:01:02 15:04:05")
	cmd := exec.Command("exiftool", "-overwrite_original", "-DateTimeOriginal="+dto, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("exiftool write failed: %v, %s", err, string(out))
	}

	cfg := &config.Config{DBPath: filepath.Join(d, "test.db"), ConvertQuality: 90, PreserveMetadata: true, MetadataStabilityDelay: 1, MD5ChunkSize: 1024}
	dbConn, err := db.Init(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md5, err := utils.MD5File(src, cfg.MD5ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	rec, _, err := db.UpsertIndex(dbConn, src, md5)
	if err != nil {
		t.Fatal(err)
	}

	conv := worker.NewConverter(cfg, dbConn)
	ctx := context.Background()
	preserved, _, _, err := conv.Convert(ctx, rec)
	if err != nil {
		t.Fatal(err)
	}
	if !preserved {
		t.Fatalf("expected DateTimeOriginal preserved")
	}

	// verify by exiftool
	heic := utils.TargetHEICPath(src)
	var out bytes.Buffer
	vcmd := exec.Command("exiftool", "-DateTimeOriginal", "-s3", heic)
	vcmd.Stdout = &out
	if err := vcmd.Run(); err != nil {
		t.Fatal(err)
	}
	val := out.String()
	if val = val[:len(val)-1]; val != dto { // trim newline
		t.Fatalf("DateTimeOriginal mismatch: %s != %s", val, dto)
	}
}
