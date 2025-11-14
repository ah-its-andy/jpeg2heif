package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// CalculateMD5 calculates the MD5 hash of a file using streaming
func CalculateMD5(filePath string, chunkSize int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()

	if chunkSize <= 0 {
		chunkSize = 8192
	}

	buffer := make([]byte, chunkSize)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read file: %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := hash.Write(buffer[:n]); err != nil {
			return "", fmt.Errorf("failed to write to hash: %w", err)
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
