package config

// Application configuration logic

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	// Flag names
	flagEnvironment = "environment"
	flagFetchSize   = "fetch-size"
	flagAppInsights = "app-insights-key"
)

// OAuth2Config holds OAuth2 authentication settings
type OAuth2Config struct {
	TenantID string   `json:"tenant_id" yaml:"tenant_id"`
	ClientID string   `json:"client_id" yaml:"client_id"`
	Scopes   []string `json:"scopes" yaml:"scopes"`
}

// Config holds application settings
type Config struct {
	mu                     *sync.RWMutex `json:"-" yaml:"-"`
	LogFetchSize           int           `json:"fetchSize" yaml:"fetchSize"`
	Environment            string        `json:"environment" yaml:"environment"`
	ApplicationInsightsKey string        `json:"applicationInsightsKey" yaml:"applicationInsightsKey"`
	OAuth2                 OAuth2Config  `json:"oauth2" yaml:"oauth2"`
}

// NewConfig creates a new Config with default values and initialized mutex
func NewConfig() Config {
	return Config{
		mu:                     &sync.RWMutex{},
		LogFetchSize:           50,
		Environment:            "Development",
		ApplicationInsightsKey: "",
		OAuth2: OAuth2Config{
			TenantID: "e48da249-7c64-41ec-8c89-cea18b6608fa",
			ClientID: "3b065ad6-067e-41f2-8cf7-19ddb0548a99",
			Scopes:   []string{"https://api.applicationinsights.io/Data.Read"},
		},
	}
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
	flagSet, flags := setupFlags()
	_ = flagSet.Parse(args)

	flagsSet := trackSetFlags(flagSet)

	// Start with default values
	cfg := NewConfig()

	// Load from configuration file if specified or found
	loadConfigFromFileIfExists(&cfg, *flags.configFile)

	// Override with environment variables
	applyEnvironmentVariables(&cfg)

	// Override with command line flags (highest priority)
	applyCommandLineFlags(&cfg, flags, flagsSet)

	return cfg
}

// setupFlags creates and configures the flag set
func setupFlags() (*flag.FlagSet, *flagValues) {
	flagSet := flag.NewFlagSet("bc-insights-tui", flag.ContinueOnError)
	flagSet.Usage = func() {} // Suppress usage output during tests

	flags := &flagValues{
		fetchSize:   flagSet.Int(flagFetchSize, -1, "Number of log entries to fetch per request"),
		environment: flagSet.String(flagEnvironment, "", "Environment name (e.g., Development, Production)"),
		appInsights: flagSet.String(flagAppInsights, "", "Application Insights connection string"),
		configFile:  flagSet.String("config", "", "Path to configuration file (JSON)"),
	}

	return flagSet, flags
}

// flagValues holds pointers to flag values
type flagValues struct {
	fetchSize   *int
	environment *string
	appInsights *string
	configFile  *string
}

// flagsSet tracks which flags were explicitly set
type flagsSet struct {
	fetchSize   bool
	environment bool
	appInsights bool
}

// trackSetFlags tracks which flags were explicitly set by the user
func trackSetFlags(flagSet *flag.FlagSet) flagsSet {
	var flags flagsSet
	flagSet.Visit(func(f *flag.Flag) {
		switch f.Name {
		case flagFetchSize:
			flags.fetchSize = true
		case flagEnvironment:
			flags.environment = true
		case flagAppInsights:
			flags.appInsights = true
		}
	})
	return flags
}

// loadConfigFromFileIfExists loads configuration from file if it exists
func loadConfigFromFileIfExists(cfg *Config, configFile string) {
	if configFile == "" {
		configFile = findConfigFile()
	}
	if configFile != "" {
		if fileConfig, err := loadConfigFromFile(configFile); err == nil {
			mergeConfig(cfg, fileConfig)
		}
		// Silently ignore file loading errors - not critical
	}
}

// applyEnvironmentVariables applies environment variable overrides
func applyEnvironmentVariables(cfg *Config) {
	if val := os.Getenv("LOG_FETCH_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.LogFetchSize = parsed
		}
	}
	if _, exists := os.LookupEnv("BCINSIGHTS_ENVIRONMENT"); exists {
		cfg.Environment = os.Getenv("BCINSIGHTS_ENVIRONMENT") // Allow empty string
	}
	if _, exists := os.LookupEnv("BCINSIGHTS_APP_INSIGHTS_KEY"); exists {
		cfg.ApplicationInsightsKey = os.Getenv("BCINSIGHTS_APP_INSIGHTS_KEY") // Allow empty string
	}

	// OAuth2 environment variables
	if _, exists := os.LookupEnv("BCINSIGHTS_OAUTH2_TENANT_ID"); exists {
		cfg.OAuth2.TenantID = os.Getenv("BCINSIGHTS_OAUTH2_TENANT_ID")
	}
	if _, exists := os.LookupEnv("BCINSIGHTS_OAUTH2_CLIENT_ID"); exists {
		cfg.OAuth2.ClientID = os.Getenv("BCINSIGHTS_OAUTH2_CLIENT_ID")
	}
	if val := os.Getenv("BCINSIGHTS_OAUTH2_SCOPES"); val != "" {
		// Split scopes by comma
		cfg.OAuth2.Scopes = strings.Split(val, ",")
		// Trim whitespace from each scope
		for i, scope := range cfg.OAuth2.Scopes {
			cfg.OAuth2.Scopes[i] = strings.TrimSpace(scope)
		}
	}
}

// applyCommandLineFlags applies command line flag overrides
func applyCommandLineFlags(cfg *Config, flags *flagValues, flagsSet flagsSet) {
	if flagsSet.fetchSize && *flags.fetchSize > 0 { // Only override if positive and explicitly set
		cfg.LogFetchSize = *flags.fetchSize
	}
	if flagsSet.environment { // Allow empty string to override if flag was explicitly set
		cfg.Environment = *flags.environment
	}
	if flagsSet.appInsights { // Allow empty string to override if flag was explicitly set
		cfg.ApplicationInsightsKey = *flags.appInsights
	}
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

	// Initialize mutex after unmarshaling
	cfg.mu = &sync.RWMutex{}

	return &cfg, nil
}

// mergeConfig merges file configuration into the base config.
func mergeConfig(base, file *Config) {
	if file.LogFetchSize > 0 {
		base.LogFetchSize = file.LogFetchSize
	}
	if file.Environment != "" {
		base.Environment = file.Environment
	}
	if file.ApplicationInsightsKey != "" {
		base.ApplicationInsightsKey = file.ApplicationInsightsKey
	}

	// Merge OAuth2 configuration
	if file.OAuth2.TenantID != "" {
		base.OAuth2.TenantID = file.OAuth2.TenantID
	}
	if file.OAuth2.ClientID != "" {
		base.OAuth2.ClientID = file.OAuth2.ClientID
	}
	if len(file.OAuth2.Scopes) > 0 {
		base.OAuth2.Scopes = file.OAuth2.Scopes
	}
}

// ValidateAndUpdateSetting validates and updates a configuration setting
func (c *Config) ValidateAndUpdateSetting(name, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch name {
	case "fetchSize":
		// Trim whitespace for parsing
		trimmed := strings.TrimSpace(value)
		if parsed, err := strconv.Atoi(trimmed); err != nil || parsed <= 0 {
			return fmt.Errorf("fetchSize must be a positive integer, got: %s", value)
		} else {
			c.LogFetchSize = parsed
		}
	case "environment":
		// Trim whitespace and check if empty (but preserve original value if valid)
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("environment cannot be empty")
		}
		c.Environment = value // Use original value, not trimmed
	case "applicationInsightsKey":
		// Allow empty for clearing the key
		c.ApplicationInsightsKey = value
	case "oauth2.tenantId":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("oauth2.tenantId cannot be empty")
		}
		c.OAuth2.TenantID = trimmed
	case "oauth2.clientId":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("oauth2.clientId cannot be empty")
		}
		c.OAuth2.ClientID = trimmed
	case "oauth2.scopes":
		// Split scopes by comma and trim whitespace
		if value == "" {
			return fmt.Errorf("oauth2.scopes cannot be empty")
		}
		scopes := strings.Split(value, ",")
		for i, scope := range scopes {
			scopes[i] = strings.TrimSpace(scope)
			if scopes[i] == "" {
				return fmt.Errorf("oauth2.scopes cannot contain empty scopes")
			}
		}
		c.OAuth2.Scopes = scopes
	default:
		return fmt.Errorf("unknown setting: %s", name)
	}
	return nil
}

// GetSettingValue returns the current value of a setting as a string
func (c *Config) GetSettingValue(name string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

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
	case "oauth2.tenantId":
		if c.OAuth2.TenantID == "" {
			return "(not set)", nil
		}
		return c.OAuth2.TenantID, nil
	case "oauth2.clientId":
		if c.OAuth2.ClientID == "" {
			return "(not set)", nil
		}
		return c.OAuth2.ClientID, nil
	case "oauth2.scopes":
		if len(c.OAuth2.Scopes) == 0 {
			return "(not set)", nil
		}
		return strings.Join(c.OAuth2.Scopes, ", "), nil
	default:
		return "", fmt.Errorf("unknown setting: %s", name)
	}
}

// ListAllSettings returns a map of all settings and their current values
func (c *Config) ListAllSettings() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

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

	// OAuth2 settings
	if c.OAuth2.TenantID == "" {
		settings["oauth2.tenantId"] = "(not set)"
	} else {
		settings["oauth2.tenantId"] = c.OAuth2.TenantID
	}
	if c.OAuth2.ClientID == "" {
		settings["oauth2.clientId"] = "(not set)"
	} else {
		settings["oauth2.clientId"] = c.OAuth2.ClientID
	}
	if len(c.OAuth2.Scopes) == 0 {
		settings["oauth2.scopes"] = "(not set)"
	} else {
		settings["oauth2.scopes"] = strings.Join(c.OAuth2.Scopes, ", ")
	}

	return settings
}
