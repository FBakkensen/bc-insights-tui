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
	flagAppID       = "app-insights-id"

	// Setting names - Basic
	settingFetchSize           = "fetchSize"
	settingEnvironment         = "environment"
	settingApplicationInsights = "applicationInsightsKey"
	settingApplicationID       = "applicationInsightsAppId"

	// Setting names - OAuth2
	settingOAuth2TenantID = "oauth2.tenantId"
	settingOAuth2ClientID = "oauth2.clientId"
	settingOAuth2Scopes   = "oauth2.scopes"

	// Setting names - KQL Editor
	settingQueryHistoryMaxEntries = "queryHistoryMaxEntries"
	settingQueryTimeoutSeconds    = "queryTimeoutSeconds"
	settingQueryHistoryFile       = "queryHistoryFile"
	settingEditorPanelRatio       = "editorPanelRatio"

	// Common strings
	notSetValue = "(not set)"

	// Internal setting keys
	settingAzureSubscriptionID = "azure.subscriptionId"

	// Config file settings
	configDirName  = "bc-insights-tui"
	configFileName = "config.json"
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
	ApplicationInsightsID  string        `json:"applicationInsightsAppId" yaml:"applicationInsightsAppId"`
	SubscriptionID         string        `json:"subscriptionId" yaml:"subscriptionId"`
	OAuth2                 OAuth2Config  `json:"oauth2" yaml:"oauth2"`
	QueryHistoryMaxEntries int           `json:"queryHistoryMaxEntries" yaml:"queryHistoryMaxEntries"`
	QueryTimeoutSeconds    int           `json:"queryTimeoutSeconds" yaml:"queryTimeoutSeconds"`
	QueryHistoryFile       string        `json:"queryHistoryFile" yaml:"queryHistoryFile"`
	EditorPanelRatio       float32       `json:"editorPanelRatio" yaml:"editorPanelRatio"`
}

// NewConfig creates a new Config with default values and initialized mutex
func NewConfig() Config {
	return Config{
		mu:                     &sync.RWMutex{},
		LogFetchSize:           50,
		Environment:            "Development",
		ApplicationInsightsKey: "",
		ApplicationInsightsID:  "",
		SubscriptionID:         "",
		OAuth2: OAuth2Config{
			TenantID: "e48da249-7c64-41ec-8c89-cea18b6608fa",
			ClientID: "3b065ad6-067e-41f2-8cf7-19ddb0548a99",
			Scopes: []string{
				"https://management.azure.com/user_impersonation",
			},
		},
		QueryHistoryMaxEntries: 100,
		QueryTimeoutSeconds:    30,
		QueryHistoryFile:       ".bc-insights-query-history.json",
		EditorPanelRatio:       0.4,
	}
}

// LoadConfig loads configuration from multiple sources in priority order:
// 1. Command line flags (highest priority)
// 2. Environment variables
// 3. Configuration files (JSON)
// 4. Default values (lowest priority)
func LoadConfig() Config {
	// Use remaining args after the application's global flag.Parse()
	// to avoid re-parsing unrelated flags (e.g., -run) in the config flag set.
	return LoadConfigWithArgs(flag.Args())
}

// LoadConfigWithArgs loads configuration with the specified command line arguments
func LoadConfigWithArgs(args []string) Config {
	loader := NewConfigLoader()
	return loader.LoadWithArgs(args)
}

// applyEnvironmentVariables applies environment variable overrides
func applyEnvironmentVariables(cfg *Config) {
	applyBasicEnvVars(cfg)
	applyOAuth2EnvVars(cfg)
	applyKQLEditorEnvVars(cfg)
}

// applyBasicEnvVars applies basic configuration environment variables
func applyBasicEnvVars(cfg *Config) {
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
	if _, exists := os.LookupEnv("BCINSIGHTS_APP_INSIGHTS_ID"); exists {
		cfg.ApplicationInsightsID = os.Getenv("BCINSIGHTS_APP_INSIGHTS_ID") // Allow empty string
	}
	// Subscription ID from common env vars
	if v := os.Getenv("AZURE_SUBSCRIPTION_ID"); v != "" {
		cfg.SubscriptionID = v
	} else if v := os.Getenv("BCINSIGHTS_AZURE_SUBSCRIPTION_ID"); v != "" {
		cfg.SubscriptionID = v
	} else if v := os.Getenv("ARM_SUBSCRIPTION_ID"); v != "" {
		cfg.SubscriptionID = v
	}
}

// applyOAuth2EnvVars applies OAuth2 configuration environment variables
func applyOAuth2EnvVars(cfg *Config) {
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

// applyKQLEditorEnvVars applies KQL Editor configuration environment variables
func applyKQLEditorEnvVars(cfg *Config) {
	if val := os.Getenv("BCINSIGHTS_QUERY_HISTORY_MAX_ENTRIES"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.QueryHistoryMaxEntries = parsed
		}
	}
	if val := os.Getenv("BCINSIGHTS_QUERY_TIMEOUT_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.QueryTimeoutSeconds = parsed
		}
	}
	if _, exists := os.LookupEnv("BCINSIGHTS_QUERY_HISTORY_FILE"); exists {
		cfg.QueryHistoryFile = os.Getenv("BCINSIGHTS_QUERY_HISTORY_FILE")
	}
	if val := os.Getenv("BCINSIGHTS_EDITOR_PANEL_RATIO"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 32); err == nil && parsed > 0 && parsed < 1 {
			cfg.EditorPanelRatio = float32(parsed)
		}
	}
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
	if file.ApplicationInsightsID != "" {
		base.ApplicationInsightsID = file.ApplicationInsightsID
	}
	if file.SubscriptionID != "" {
		base.SubscriptionID = file.SubscriptionID
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

	// Merge KQL Editor configuration
	if file.QueryHistoryMaxEntries > 0 {
		base.QueryHistoryMaxEntries = file.QueryHistoryMaxEntries
	}
	if file.QueryTimeoutSeconds > 0 {
		base.QueryTimeoutSeconds = file.QueryTimeoutSeconds
	}
	if file.QueryHistoryFile != "" {
		base.QueryHistoryFile = file.QueryHistoryFile
	}
	if file.EditorPanelRatio > 0 && file.EditorPanelRatio < 1 {
		base.EditorPanelRatio = file.EditorPanelRatio
	}
}

// ValidateAndUpdateSetting validates and updates a configuration setting
func (c *Config) ValidateAndUpdateSetting(name, value string) error {
	// First, validate and update the setting
	c.mu.Lock()
	var err error
	if c.isBasicSetting(name) {
		err = c.validateBasicSetting(name, value)
	} else if c.isOAuth2Setting(name) {
		err = c.validateOAuth2Setting(name, value)
	} else if c.isKQLEditorSetting(name) {
		err = c.validateKQLEditorSetting(name, value)
	} else {
		c.mu.Unlock()
		return fmt.Errorf("unknown setting: %s", name)
	}
	c.mu.Unlock()

	if err != nil {
		return err
	}

	// Setting updated successfully, now save to file
	if saveErr := c.SaveConfig(); saveErr != nil {
		// Log the error but don't fail the update - the in-memory change is still valid
		// Return a wrapped error to inform the caller about the save issue
		return fmt.Errorf("setting updated in memory but failed to save to file: %w", saveErr)
	}

	return nil
}

// isBasicSetting checks if the setting name is a basic configuration setting
func (c *Config) isBasicSetting(name string) bool {
	switch name {
	case settingFetchSize, settingEnvironment, settingApplicationInsights, settingApplicationID, settingAzureSubscriptionID:
		return true
	default:
		return false
	}
}

// isOAuth2Setting checks if the setting name is an OAuth2 configuration setting
func (c *Config) isOAuth2Setting(name string) bool {
	switch name {
	case settingOAuth2TenantID, settingOAuth2ClientID, settingOAuth2Scopes:
		return true
	default:
		return false
	}
}

// isKQLEditorSetting checks if the setting name is a KQL Editor configuration setting
func (c *Config) isKQLEditorSetting(name string) bool {
	switch name {
	case settingQueryHistoryMaxEntries, settingQueryTimeoutSeconds, settingQueryHistoryFile, settingEditorPanelRatio:
		return true
	default:
		return false
	}
}

// validateBasicSetting validates and updates basic configuration settings
func (c *Config) validateBasicSetting(name, value string) error {
	switch name {
	case settingFetchSize:
		// Trim whitespace for parsing
		trimmed := strings.TrimSpace(value)
		if parsed, err := strconv.Atoi(trimmed); err != nil || parsed <= 0 {
			return fmt.Errorf("fetchSize must be a positive integer, got: %s", value)
		} else {
			c.LogFetchSize = parsed
		}
	case settingEnvironment:
		// Trim whitespace and check if empty (but preserve original value if valid)
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("environment cannot be empty")
		}
		c.Environment = value // Use original value, not trimmed
	case settingApplicationInsights:
		// Allow empty for clearing the key
		c.ApplicationInsightsKey = value
	case settingApplicationID:
		// Allow empty for clearing the ID
		c.ApplicationInsightsID = value
	case settingAzureSubscriptionID:
		// Basic non-empty validation
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("azure.subscriptionId cannot be empty")
		}
		c.SubscriptionID = trimmed
	default:
		return fmt.Errorf("unknown basic setting: %s", name)
	}
	return nil
}

// validateOAuth2Setting validates and updates OAuth2 configuration settings
func (c *Config) validateOAuth2Setting(name, value string) error {
	switch name {
	case settingOAuth2TenantID:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("oauth2.tenantId cannot be empty")
		}
		c.OAuth2.TenantID = trimmed
	case settingOAuth2ClientID:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("oauth2.clientId cannot be empty")
		}
		c.OAuth2.ClientID = trimmed
	case settingOAuth2Scopes:
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
		return fmt.Errorf("unknown oauth2 setting: %s", name)
	}
	return nil
}

// validateKQLEditorSetting validates and updates KQL Editor configuration settings
func (c *Config) validateKQLEditorSetting(name, value string) error {
	switch name {
	case settingQueryHistoryMaxEntries:
		trimmed := strings.TrimSpace(value)
		if parsed, err := strconv.Atoi(trimmed); err != nil || parsed <= 0 {
			return fmt.Errorf("queryHistoryMaxEntries must be a positive integer, got: %s", value)
		} else {
			c.QueryHistoryMaxEntries = parsed
		}
	case settingQueryTimeoutSeconds:
		trimmed := strings.TrimSpace(value)
		if parsed, err := strconv.Atoi(trimmed); err != nil || parsed <= 0 {
			return fmt.Errorf("queryTimeoutSeconds must be a positive integer, got: %s", value)
		} else {
			c.QueryTimeoutSeconds = parsed
		}
	case settingQueryHistoryFile:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("queryHistoryFile cannot be empty")
		}
		c.QueryHistoryFile = trimmed
	case settingEditorPanelRatio:
		trimmed := strings.TrimSpace(value)
		if parsed, err := strconv.ParseFloat(trimmed, 32); err != nil || parsed <= 0 || parsed >= 1 {
			return fmt.Errorf("editorPanelRatio must be a decimal between 0 and 1, got: %s", value)
		} else {
			c.EditorPanelRatio = float32(parsed)
		}
	default:
		return fmt.Errorf("unknown kql editor setting: %s", name)
	}
	return nil
}

// GetSettingValue returns the current value of a setting as a string
func (c *Config) GetSettingValue(name string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Try basic settings first
	if c.isBasicSetting(name) {
		return c.getBasicSettingValue(name)
	}

	// Try OAuth2 settings
	if c.isOAuth2Setting(name) {
		return c.getOAuth2SettingValue(name)
	}

	// Try KQL Editor settings
	if c.isKQLEditorSetting(name) {
		return c.getKQLEditorSettingValue(name)
	}

	return "", fmt.Errorf("unknown setting: %s", name)
}

// getBasicSettingValue returns the value of basic configuration settings
func (c *Config) getBasicSettingValue(name string) (string, error) {
	switch name {
	case settingFetchSize:
		return strconv.Itoa(c.LogFetchSize), nil
	case settingEnvironment:
		return c.Environment, nil
	case settingApplicationInsights:
		if c.ApplicationInsightsKey == "" {
			return notSetValue, nil
		}
		// Mask the key for display
		if len(c.ApplicationInsightsKey) > 8 {
			return c.ApplicationInsightsKey[:4] + "..." + c.ApplicationInsightsKey[len(c.ApplicationInsightsKey)-4:], nil
		}
		return "***", nil
	case settingApplicationID:
		if c.ApplicationInsightsID == "" {
			return notSetValue, nil
		}
		return c.ApplicationInsightsID, nil
	case settingAzureSubscriptionID:
		if c.SubscriptionID == "" {
			return notSetValue, nil
		}
		return c.SubscriptionID, nil
	default:
		return "", fmt.Errorf("unknown basic setting: %s", name)
	}
}

// getOAuth2SettingValue returns the value of OAuth2 configuration settings
func (c *Config) getOAuth2SettingValue(name string) (string, error) {
	switch name {
	case settingOAuth2TenantID:
		if c.OAuth2.TenantID == "" {
			return notSetValue, nil
		}
		return c.OAuth2.TenantID, nil
	case settingOAuth2ClientID:
		if c.OAuth2.ClientID == "" {
			return notSetValue, nil
		}
		return c.OAuth2.ClientID, nil
	case settingOAuth2Scopes:
		if len(c.OAuth2.Scopes) == 0 {
			return notSetValue, nil
		}
		return strings.Join(c.OAuth2.Scopes, ", "), nil
	default:
		return "", fmt.Errorf("unknown oauth2 setting: %s", name)
	}
}

// getKQLEditorSettingValue returns the value of KQL Editor configuration settings
func (c *Config) getKQLEditorSettingValue(name string) (string, error) {
	switch name {
	case settingQueryHistoryMaxEntries:
		return strconv.Itoa(c.QueryHistoryMaxEntries), nil
	case settingQueryTimeoutSeconds:
		return strconv.Itoa(c.QueryTimeoutSeconds), nil
	case settingQueryHistoryFile:
		if c.QueryHistoryFile == "" {
			return notSetValue, nil
		}
		return c.QueryHistoryFile, nil
	case settingEditorPanelRatio:
		return fmt.Sprintf("%.2f", c.EditorPanelRatio), nil
	default:
		return "", fmt.Errorf("unknown kql editor setting: %s", name)
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
		settings["applicationInsightsKey"] = notSetValue
	} else if len(c.ApplicationInsightsKey) > 8 {
		settings["applicationInsightsKey"] = c.ApplicationInsightsKey[:4] + "..." + c.ApplicationInsightsKey[len(c.ApplicationInsightsKey)-4:]
	} else {
		settings["applicationInsightsKey"] = "***"
	}
	if c.ApplicationInsightsID == "" {
		settings["applicationInsightsAppId"] = notSetValue
	} else {
		settings["applicationInsightsAppId"] = c.ApplicationInsightsID
	}
	if c.SubscriptionID == "" {
		settings[settingAzureSubscriptionID] = notSetValue
	} else {
		settings[settingAzureSubscriptionID] = c.SubscriptionID
	}

	// OAuth2 settings
	if c.OAuth2.TenantID == "" {
		settings["oauth2.tenantId"] = notSetValue
	} else {
		settings["oauth2.tenantId"] = c.OAuth2.TenantID
	}
	if c.OAuth2.ClientID == "" {
		settings["oauth2.clientId"] = notSetValue
	} else {
		settings["oauth2.clientId"] = c.OAuth2.ClientID
	}
	if len(c.OAuth2.Scopes) == 0 {
		settings["oauth2.scopes"] = notSetValue
	} else {
		settings["oauth2.scopes"] = strings.Join(c.OAuth2.Scopes, ", ")
	}

	// KQL Editor settings
	settings["queryHistoryMaxEntries"] = strconv.Itoa(c.QueryHistoryMaxEntries)
	settings["queryTimeoutSeconds"] = strconv.Itoa(c.QueryTimeoutSeconds)
	if c.QueryHistoryFile == "" {
		settings["queryHistoryFile"] = notSetValue
	} else {
		settings["queryHistoryFile"] = c.QueryHistoryFile
	}
	settings["editorPanelRatio"] = fmt.Sprintf("%.2f", c.EditorPanelRatio)

	return settings
}

// isTestMode checks if we're running in test mode to enable test isolation
func isTestMode() bool {
	// Check for Go test environment
	return flag.Lookup("test.v") != nil
}

// getConfigFilePath returns the full path to the config file
func getConfigFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	appConfigDir := filepath.Join(configDir, configDirName)
	configPath := filepath.Join(appConfigDir, configFileName)

	return configPath, nil
}

// SaveConfig saves the current configuration to a JSON file atomically
func (c *Config) SaveConfig() error {
	// Don't save config files during tests to maintain test isolation
	if isTestMode() {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	configPath, err := getConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to determine config file path: %w", err)
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	err = os.MkdirAll(configDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	// Create temporary file in the same directory for atomic write
	tempFile, err := os.CreateTemp(configDir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name()) // Clean up temp file on error
	}()

	// Encode config to JSON with pretty printing
	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config to JSON: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}

	// Atomically replace the config file
	if err := os.Rename(tempFile.Name(), configPath); err != nil {
		return fmt.Errorf("failed to save config file %s: %w", configPath, err)
	}

	return nil
}
