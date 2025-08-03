package config

import (
	"os"
	"testing"
)

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Ensure environment is clean
	if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
		t.Logf("Failed to unset environment variable: %v", err)
	}

	cfg := LoadConfig()

	if cfg.LogFetchSize != 50 {
		t.Errorf("Expected default LogFetchSize to be 50, got %d", cfg.LogFetchSize)
	}
}

func TestLoadConfig_ValidEnvironmentVariable(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected int
	}{
		{"small value", "1", 1},
		{"medium value", "100", 100},
		{"large value", "999", 999},
		{"string number", "200", 200},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable
			if err := os.Setenv("LOG_FETCH_SIZE", tc.envValue); err != nil {
				t.Fatalf("Failed to set environment variable: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
					t.Logf("Failed to unset environment variable: %v", err)
				}
			}()

			cfg := LoadConfig()

			if cfg.LogFetchSize != tc.expected {
				t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expected, cfg.LogFetchSize)
			}
		})
	}
}

func TestLoadConfig_InvalidEnvironmentVariable(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected int // Should fallback to default
	}{
		{"empty string", "", 50},
		{"non-numeric", "invalid", 50},
		{"negative number", "-10", 50},
		{"zero", "0", 50},
		{"float", "10.5", 50},
		{"special chars", "!@#", 50},
		{"mixed alphanumeric", "10abc", 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable
			if err := os.Setenv("LOG_FETCH_SIZE", tc.envValue); err != nil {
				t.Fatalf("Failed to set environment variable: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
					t.Logf("Failed to unset environment variable: %v", err)
				}
			}()

			cfg := LoadConfig()

			if cfg.LogFetchSize != tc.expected {
				t.Errorf("Expected LogFetchSize to fallback to %d for invalid value %q, got %d",
					tc.expected, tc.envValue, cfg.LogFetchSize)
			}
		})
	}
}

func TestLoadConfig_EnvironmentVariableEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected int
	}{
		{"leading spaces", " 42", 50},           // Invalid due to spaces
		{"trailing spaces", "42 ", 50},          // Invalid due to spaces
		{"leading zero", "042", 42},             // Valid - Go parses as decimal
		{"very large number", "999999", 999999}, // Valid large number
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable
			if err := os.Setenv("LOG_FETCH_SIZE", tc.envValue); err != nil {
				t.Fatalf("Failed to set environment variable: %v", err)
			}
			defer func() {
				if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
					t.Logf("Failed to unset environment variable: %v", err)
				}
			}()

			cfg := LoadConfig()

			if cfg.LogFetchSize != tc.expected {
				t.Errorf("Expected LogFetchSize to be %d for value %q, got %d",
					tc.expected, tc.envValue, cfg.LogFetchSize)
			}
		})
	}
}

func TestConfig_StructFields(t *testing.T) {
	// Test that Config struct has expected fields
	cfg := Config{LogFetchSize: 123}

	if cfg.LogFetchSize != 123 {
		t.Errorf("Expected LogFetchSize field to be settable, got %d", cfg.LogFetchSize)
	}
}
