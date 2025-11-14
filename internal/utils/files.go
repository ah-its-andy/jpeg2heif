package utils

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func IsJPEG(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg"
}

func WaitFileStable(path string, delay time.Duration) error {
	// Wait for two consecutive identical sizes separated by delay
	var lastSize int64 = -1
	for i := 0; i < 5; i++ { // up to ~5 cycles
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		sz := fi.Size()
		if lastSize == sz {
			return nil
		}
		lastSize = sz
		time.Sleep(delay)
	}
	return nil
}

func TargetHEICPath(src string) string {
	base := filepath.Base(src)
	name := strings.TrimSuffix(base, filepath.Ext(base)) + ".heic"
	d := filepath.Dir(src)    // .../b/c
	parent := filepath.Dir(d) // .../b
	outDir := filepath.Join(parent, "heic")
	return filepath.Join(outDir, name)
}
