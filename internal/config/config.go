package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	WatchDirs              []string
	DBPath                 string
	LogLevel               string
	PollIntervalSec        int
	MaxWorkers             int
	ConvertQuality         int
	HTTPPort               int
	PreserveMetadata       bool
	MetadataStabilityDelay int
	MD5ChunkSize           int64
}

func Load() *Config {
	cfg := &Config{}
	cfg.WatchDirs = splitAndTrim(os.Getenv("WATCH_DIRS"))
	cfg.DBPath = getEnv("DB_PATH", "/data/tasks.db")
	cfg.LogLevel = getEnv("LOG_LEVEL", "INFO")
	cfg.PollIntervalSec = getEnvInt("POLL_INTERVAL", 1)
	cfg.MaxWorkers = getEnvInt("MAX_WORKERS", 4)
	cfg.ConvertQuality = getEnvInt("CONVERT_QUALITY", 90)
	cfg.HTTPPort = getEnvInt("HTTP_PORT", 8000)
	cfg.PreserveMetadata = getEnvBool("PRESERVE_METADATA", true)
	cfg.MetadataStabilityDelay = getEnvInt("METADATA_STABILITY_DELAY", 1)
	cfg.MD5ChunkSize = getEnvInt64("MD5_CHUNK_SIZE", 4*1024*1024)
	return cfg
}

func (c *Config) HTTPAddr() string { return fmt.Sprintf(":%d", c.HTTPPort) }

func splitAndTrim(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getEnvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return i
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
