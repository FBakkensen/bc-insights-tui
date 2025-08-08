package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
)

// FlagParser abstracts command line flag parsing for testability
type FlagParser interface {
	Parse(args []string) *ParsedFlags
}

// ParsedFlags represents parsed command line flags
type ParsedFlags struct {
	configFile            *string
	environment           *string
	fetchSize             *int
	applicationInsights   *string
	applicationInsightsID *string
}

// OsFlagParser implements FlagParser using the real flag package
type OsFlagParser struct{}

// Parse implements FlagParser.Parse using the real flag package
func (fp *OsFlagParser) Parse(args []string) *ParsedFlags {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)

	// Define flags
	configFile := fs.String("config", "", "Configuration file path")
	environment := fs.String(flagEnvironment, "", "Environment name")
	fetchSize := fs.Int(flagFetchSize, 0, "Number of log entries to fetch")
	applicationInsights := fs.String(flagAppInsights, "", "Application Insights instrumentation key")
	applicationInsightsID := fs.String(flagAppID, "", "Application Insights application ID")

	_ = fs.Parse(args) // Ignore parse errors for now

	flags := &ParsedFlags{}

	// Only set pointers if flags were actually provided
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "config":
			flags.configFile = configFile
		case flagEnvironment:
			flags.environment = environment
		case flagFetchSize:
			flags.fetchSize = fetchSize
		case flagAppInsights:
			flags.applicationInsights = applicationInsights
		case flagAppID:
			flags.applicationInsightsID = applicationInsightsID
		}
	})

	return flags
}

// ConfigLoader handles configuration loading with injected dependencies
type ConfigLoader struct {
	fs          FileSystem
	flagParser  FlagParser
	searchPaths []string
}

// NewConfigLoader creates a ConfigLoader for production use
func NewConfigLoader() *ConfigLoader {
	osFS := &OsFileSystem{}
	searchPaths := getDefaultSearchPaths(osFS)

	return &ConfigLoader{
		fs:          osFS,
		flagParser:  &OsFlagParser{},
		searchPaths: searchPaths,
	}
}

// NewTestConfigLoader creates a ConfigLoader for testing with custom dependencies
func NewTestConfigLoader(fs FileSystem, searchPaths []string) *ConfigLoader {
	return &ConfigLoader{
		fs:          fs,
		flagParser:  &MockFlagParser{},
		searchPaths: searchPaths,
	}
}

// Load loads configuration using the default arguments
func (cl *ConfigLoader) Load() Config {
	// In production, we need to get the real args
	// For testing, this won't be called directly
	return cl.LoadWithArgs(flag.Args())
}

// LoadWithArgs loads configuration with the specified command line arguments
func (cl *ConfigLoader) LoadWithArgs(args []string) Config {
	cfg := NewConfig()

	// Parse command line flags
	flags := cl.flagParser.Parse(args)

	// Load from file (file takes precedence over defaults)
	cl.loadFromFile(&cfg, flags.configFile)

	// Load from environment variables (env takes precedence over file)
	cl.loadFromEnv(&cfg)

	// Apply command line flags (flags take precedence over everything)
	cl.applyFlags(&cfg, flags)

	return cfg
}

// loadFromFile loads configuration from file using the injected filesystem
func (cl *ConfigLoader) loadFromFile(cfg *Config, configFile *string) {
	var filePath string

	if configFile != nil && *configFile != "" {
		filePath = *configFile
	} else {
		filePath = cl.findConfigFile()
	}

	if filePath != "" {
		if fileConfig, err := cl.loadConfigFromFile(filePath); err == nil {
			mergeConfig(cfg, fileConfig)
		}
	}
}

// findConfigFile looks for configuration files in the search paths
func (cl *ConfigLoader) findConfigFile() string {
	for _, path := range cl.searchPaths {
		if _, err := cl.fs.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// loadConfigFromFile loads configuration from a JSON file using the injected filesystem
func (cl *ConfigLoader) loadConfigFromFile(filename string) (*Config, error) {
	data, err := cl.fs.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	return &cfg, nil
}

// loadFromEnv loads configuration from environment variables
func (cl *ConfigLoader) loadFromEnv(cfg *Config) {
	// This doesn't need filesystem access, so we can use the existing logic
	applyEnvironmentVariables(cfg)
}

// applyFlags applies command line flags to the configuration
func (cl *ConfigLoader) applyFlags(cfg *Config, flags *ParsedFlags) {
	if flags.environment != nil {
		cfg.Environment = *flags.environment // Allow empty string to override
	}
	if flags.fetchSize != nil && *flags.fetchSize > 0 {
		cfg.LogFetchSize = *flags.fetchSize
	}
	if flags.applicationInsights != nil && *flags.applicationInsights != "" {
		cfg.ApplicationInsightsKey = *flags.applicationInsights
	}
	if flags.applicationInsightsID != nil && *flags.applicationInsightsID != "" {
		cfg.ApplicationInsightsID = *flags.applicationInsightsID
	}
}

// getDefaultSearchPaths returns the default search paths for config files
func getDefaultSearchPaths(fs FileSystem) []string {
	var paths []string

	// Current directory files
	if cwd, err := fs.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "config.json"))
		paths = append(paths, filepath.Join(cwd, "bc-insights-tui.json"))
	}

	// User config directory (where SaveConfig saves files) - HIGHEST PRIORITY
	if configDir, err := fs.UserConfigDir(); err == nil {
		appConfigDir := filepath.Join(configDir, "bc-insights-tui")
		paths = append(paths, filepath.Join(appConfigDir, "config.json"))
		paths = append(paths, filepath.Join(appConfigDir, "bc-insights-tui.json"))
	}

	// Home directory files (legacy support)
	if home, err := fs.UserHomeDir(); err == nil {
		// User's app directory
		appDir := filepath.Join(home, ".bc-insights-tui")
		paths = append(paths, filepath.Join(appDir, "config.json"))
		paths = append(paths, filepath.Join(appDir, "bc-insights-tui.json"))

		// Direct home directory
		paths = append(paths, filepath.Join(home, "config.json"))
		paths = append(paths, filepath.Join(home, "bc-insights-tui.json"))
	}

	return paths
}

// MockFlagParser implements FlagParser for testing
type MockFlagParser struct {
	flags *ParsedFlags
}

// SetFlags allows tests to set mock flag values
func (mfp *MockFlagParser) SetFlags(flags *ParsedFlags) {
	mfp.flags = flags
}

// Parse implements FlagParser.Parse returning mock values
func (mfp *MockFlagParser) Parse(args []string) *ParsedFlags {
	if mfp.flags != nil {
		return mfp.flags
	}

	// Return empty flags with nil pointers when none are set
	return &ParsedFlags{
		configFile:            nil,
		environment:           nil,
		fetchSize:             nil,
		applicationInsights:   nil,
		applicationInsightsID: nil,
	}
}
