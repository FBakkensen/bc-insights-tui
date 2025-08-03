package config

// Application configuration logic

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds application settings
type Config struct {
	LogFetchSize           int    `json:"fetchSize" yaml:"fetchSize"`
	Environment            string `json:"environment" yaml:"environment"`
	ApplicationInsightsKey string `json:"applicationInsightsKey" yaml:"applicationInsightsKey"`
}

// LoadConfig loads configuration from multiple sources in priority order:
// 1. Command line flags (highest priority)
// 2. Environment variables
// 3. Configuration files (JSON)
// 4. Default values (lowest priority)
func LoadConfig() Config {
	return LoadConfigWithArgs(os.Args[1:])
}

// LoadConfigWithArgs loads configuration with the specified command line arguments
func LoadConfigWithArgs(args []string) Config {
	// Create a new flag set to avoid conflicts during testing
	flagSet := flag.NewFlagSet("bc-insights-tui", flag.ContinueOnError)
	flagSet.Usage = func() {} // Suppress usage output during tests

	var (
		fetchSizeFlag   = flagSet.Int("fetch-size", 0, "Number of log entries to fetch per request")
		environmentFlag = flagSet.String("environment", "", "Environment name (e.g., Development, Production)")
		appInsightsFlag = flagSet.String("app-insights-key", "", "Application Insights connection string")
		configFileFlag  = flagSet.String("config", "", "Path to configuration file (JSON)")
	)

	// Parse arguments, ignoring errors (for compatibility)
	flagSet.Parse(args)

	// Start with default values
	cfg := Config{
		LogFetchSize:           50,
		Environment:            "Development",
		ApplicationInsightsKey: "",
	}

	// Load from configuration file if specified or found
	configFile := *configFileFlag
	if configFile == "" {
		// Look for config files in standard locations
		configFile = findConfigFile()
	}
	if configFile != "" {
		if fileConfig, err := loadConfigFromFile(configFile); err == nil {
			mergeConfig(&cfg, fileConfig)
		}
		// Silently ignore file loading errors - not critical
	}

	// Override with environment variables
	if val := os.Getenv("LOG_FETCH_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.LogFetchSize = parsed
		}
	}
	if val := os.Getenv("BCINSIGHTS_ENVIRONMENT"); val != "" {
		cfg.Environment = val
	}
	if val := os.Getenv("BCINSIGHTS_APP_INSIGHTS_KEY"); val != "" {
		cfg.ApplicationInsightsKey = val
	}

	// Override with command line flags (highest priority)
	if *fetchSizeFlag > 0 {
		cfg.LogFetchSize = *fetchSizeFlag
	}
	if *environmentFlag != "" {
		cfg.Environment = *environmentFlag
	}
	if *appInsightsFlag != "" {
		cfg.ApplicationInsightsKey = *appInsightsFlag
	}

	return cfg
}

// findConfigFile looks for configuration files in standard locations
func findConfigFile() string {
	// Check current directory first
	candidates := []string{
		"config.json",
		"bc-insights-tui.json",
	}

	// Add home directory candidates
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(homeDir, ".bc-insights-tui", "config.json"),
			filepath.Join(homeDir, ".bc-insights-tui.json"),
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// loadConfigFromFile loads configuration from a JSON file
func loadConfigFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// mergeConfig merges file configuration into the base config
func mergeConfig(base *Config, file *Config) {
	if file.LogFetchSize > 0 {
		base.LogFetchSize = file.LogFetchSize
	}
	if file.Environment != "" {
		base.Environment = file.Environment
	}
	if file.ApplicationInsightsKey != "" {
		base.ApplicationInsightsKey = file.ApplicationInsightsKey
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
