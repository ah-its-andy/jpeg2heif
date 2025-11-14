package tests

import (
	"os"
	"path/filepath"
	"testing"

	"image"
	"image/color"
	"image/jpeg"

	"github.com/ah-its-andy/jpeg2heif/internal/config"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/utils"
)

func TestIndexAndMD5(t *testing.T) {
	d := t.TempDir()
	imgDir := filepath.Join(d, "a", "b", "c")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(imgDir, "test.jpg")
	createJPEG(t, imgPath)

	cfg := &config.Config{DBPath: filepath.Join(d, "test.db"), MD5ChunkSize: 1024}
	dbConn, err := db.Init(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md5, err := utils.MD5File(imgPath, cfg.MD5ChunkSize)
	if err != nil {
		t.Fatal(err)
	}
	rec, changed, err := db.UpsertIndex(dbConn, imgPath, md5)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("expected changed true on first insert")
	}
	if rec.FileMD5 != md5 {
		t.Fatalf("md5 mismatch: %s != %s", rec.FileMD5, md5)
	}
}

func createJPEG(t *testing.T, path string) {
	rect := image.Rect(0, 0, 10, 10)
	img := image.NewRGBA(rect)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 20), uint8(y * 20), 0, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
}
