package config

// Application configuration logic

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application settings
type Config struct {
	LogFetchSize           int    `json:"fetchSize" yaml:"fetchSize"`
	Environment            string `json:"environment" yaml:"environment"`
	ApplicationInsightsKey string `json:"applicationInsightsKey" yaml:"applicationInsightsKey"`
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

	// Default environment
	environment := "Development"
	if val := os.Getenv("BCINSIGHTS_ENVIRONMENT"); val != "" {
		environment = val
	}

	// Application Insights key (no default)
	appInsightsKey := os.Getenv("BCINSIGHTS_APP_INSIGHTS_KEY")

	return Config{
		LogFetchSize:           fetchSize,
		Environment:            environment,
		ApplicationInsightsKey: appInsightsKey,
	}
}

// ValidateAndUpdateSetting validates and updates a configuration setting
func (c *Config) ValidateAndUpdateSetting(name, value string) error {
	switch name {
	case "fetchSize":
		if parsed, err := strconv.Atoi(value); err != nil || parsed <= 0 {
			return fmt.Errorf("fetchSize must be a positive integer, got: %s", value)
		} else {
			c.LogFetchSize = parsed
		}
	case "environment":
		if value == "" {
			return fmt.Errorf("environment cannot be empty")
		}
		c.Environment = value
	case "applicationInsightsKey":
		// Allow empty for clearing the key
		c.ApplicationInsightsKey = value
	default:
		return fmt.Errorf("unknown setting: %s", name)
	}
	return nil
}

// GetSettingValue returns the current value of a setting as a string
func (c *Config) GetSettingValue(name string) (string, error) {
	switch name {
	case "fetchSize":
		return strconv.Itoa(c.LogFetchSize), nil
	case "environment":
		return c.Environment, nil
	case "applicationInsightsKey":
		if c.ApplicationInsightsKey == "" {
			return "(not set)", nil
		}
		// Mask the key for display
		if len(c.ApplicationInsightsKey) > 8 {
			return c.ApplicationInsightsKey[:4] + "..." + c.ApplicationInsightsKey[len(c.ApplicationInsightsKey)-4:], nil
		}
		return "***", nil
	default:
		return "", fmt.Errorf("unknown setting: %s", name)
	}
}

// ListAllSettings returns a map of all settings and their current values
func (c *Config) ListAllSettings() map[string]string {
	settings := make(map[string]string)
	settings["fetchSize"] = strconv.Itoa(c.LogFetchSize)
	settings["environment"] = c.Environment
	if c.ApplicationInsightsKey == "" {
		settings["applicationInsightsKey"] = "(not set)"
	} else if len(c.ApplicationInsightsKey) > 8 {
		settings["applicationInsightsKey"] = c.ApplicationInsightsKey[:4] + "..." + c.ApplicationInsightsKey[len(c.ApplicationInsightsKey)-4:]
	} else {
		settings["applicationInsightsKey"] = "***"
	}
	return settings
}
