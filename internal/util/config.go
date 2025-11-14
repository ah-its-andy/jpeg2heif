package util

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds application configuration
type Config struct {
	WatchDirs              []string
	DBPath                 string
	HTTPPort               int
	LogLevel               string
	PollInterval           time.Duration
	MetadataStabilityDelay time.Duration
	MaxWorkers             int
	ConvertQuality         int
	PreserveMetadata       bool
	MD5ChunkSize           int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		WatchDirs:              parseWatchDirs(getEnv("WATCH_DIRS", "/data/watch")),
		DBPath:                 getEnv("DB_PATH", "/data/jpeg2heif.db"),
		HTTPPort:               getEnvInt("HTTP_PORT", 8080),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		PollInterval:           getEnvDuration("POLL_INTERVAL", 300*time.Second),
		MetadataStabilityDelay: getEnvDuration("METADATA_STABILITY_DELAY", 5*time.Second),
		MaxWorkers:             getEnvInt("MAX_WORKERS", 4),
		ConvertQuality:         getEnvInt("CONVERT_QUALITY", 85),
		PreserveMetadata:       getEnvBool("PRESERVE_METADATA", true),
		MD5ChunkSize:           getEnvInt("MD5_CHUNK_SIZE", 8192),
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: invalid integer value for %s: %s, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		log.Printf("Warning: invalid duration value for %s: %s, using default: %v", key, value, defaultValue)
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		log.Printf("Warning: invalid boolean value for %s: %s, using default: %t", key, value, defaultValue)
	}
	return defaultValue
}

func parseWatchDirs(dirs string) []string {
	parts := strings.Split(dirs, ",")
	result := []string{}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
