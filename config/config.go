package config

// Application configuration logic

import (
	"os"
	"strconv"
)

// Config holds application settings
type Config struct {
	LogFetchSize int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() Config {
	// Default fetch size
	fetchSize := 50
	if val := os.Getenv("LOG_FETCH_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			fetchSize = parsed
		}
		// If parsing fails or value is invalid, fallback to default
	}
	return Config{LogFetchSize: fetchSize}
}
