package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// Additional tests as requested in the issue

func TestLoadConfig_CommandLineFlags(t *testing.T) {
	// Test command line flags override environment variables
	testCases := []struct {
		name     string
		args     []string
		envVars  map[string]string
		expected Config
	}{
		{
			name:    "fetch-size flag overrides env",
			args:    []string{"--fetch-size=200"},
			envVars: map[string]string{"LOG_FETCH_SIZE": "100"},
			expected: Config{
				LogFetchSize:           200,
				Environment:            "Development",
				ApplicationInsightsKey: "",
			},
		},
		{
			name:    "environment flag overrides env",
			args:    []string{"--environment=Testing"},
			envVars: map[string]string{"BCINSIGHTS_ENVIRONMENT": "Production"},
			expected: Config{
				LogFetchSize:           50,
				Environment:            "Testing",
				ApplicationInsightsKey: "",
			},
		},
		{
			name:    "app-insights-key flag overrides env",
			args:    []string{"--app-insights-key=flag-key"},
			envVars: map[string]string{"BCINSIGHTS_APP_INSIGHTS_KEY": "env-key"},
			expected: Config{
				LogFetchSize:           50,
				Environment:            "Development",
				ApplicationInsightsKey: "flag-key",
			},
		},
		{
			name: "multiple flags override multiple env vars",
			args: []string{"--fetch-size=300", "--environment=Staging", "--app-insights-key=multi-key"},
			envVars: map[string]string{
				"LOG_FETCH_SIZE":              "100",
				"BCINSIGHTS_ENVIRONMENT":      "Production",
				"BCINSIGHTS_APP_INSIGHTS_KEY": "env-key",
			},
			expected: Config{
				LogFetchSize:           300,
				Environment:            "Staging",
				ApplicationInsightsKey: "multi-key",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tc.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			cfg := LoadConfigWithArgs(tc.args)

			if cfg.LogFetchSize != tc.expected.LogFetchSize {
				t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expected.LogFetchSize, cfg.LogFetchSize)
			}
			if cfg.Environment != tc.expected.Environment {
				t.Errorf("Expected Environment to be %q, got %q", tc.expected.Environment, cfg.Environment)
			}
			if cfg.ApplicationInsightsKey != tc.expected.ApplicationInsightsKey {
				t.Errorf("Expected ApplicationInsightsKey to be %q, got %q", tc.expected.ApplicationInsightsKey, cfg.ApplicationInsightsKey)
			}
		})
	}
}

func TestLoadConfig_ConfigFileLoading(t *testing.T) {
	// Create temporary directory for test config files
	tempDir := t.TempDir()

	testCases := []struct {
		name        string
		fileContent string
		fileName    string
		expectedCfg Config
		shouldError bool
	}{
		{
			name: "valid JSON config",
			fileContent: `{
				"fetchSize": 150,
				"environment": "TestEnv",
				"applicationInsightsKey": "test-key"
			}`,
			fileName: "test-config.json",
			expectedCfg: Config{
				LogFetchSize:           150,
				Environment:            "TestEnv",
				ApplicationInsightsKey: "test-key",
			},
		},
		{
			name: "partial JSON config",
			fileContent: `{
				"fetchSize": 75
			}`,
			fileName: "partial-config.json",
			expectedCfg: Config{
				LogFetchSize:           75,
				Environment:            "Development", // default
				ApplicationInsightsKey: "",            // default
			},
		},
		{
			name:        "empty JSON config",
			fileContent: `{}`,
			fileName:    "empty-config.json",
			expectedCfg: Config{
				LogFetchSize:           50,            // default
				Environment:            "Development", // default
				ApplicationInsightsKey: "",            // default
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test config file
			configPath := filepath.Join(tempDir, tc.fileName)
			err := os.WriteFile(configPath, []byte(tc.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			// Load config with explicit file path
			cfg := LoadConfigWithArgs([]string{"--config=" + configPath})

			if cfg.LogFetchSize != tc.expectedCfg.LogFetchSize {
				t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expectedCfg.LogFetchSize, cfg.LogFetchSize)
			}
			if cfg.Environment != tc.expectedCfg.Environment {
				t.Errorf("Expected Environment to be %q, got %q", tc.expectedCfg.Environment, cfg.Environment)
			}
			if cfg.ApplicationInsightsKey != tc.expectedCfg.ApplicationInsightsKey {
				t.Errorf("Expected ApplicationInsightsKey to be %q, got %q", tc.expectedCfg.ApplicationInsightsKey, cfg.ApplicationInsightsKey)
			}
		})
	}
}

func TestLoadConfig_ConfigFileLocations(t *testing.T) {
	// Test config file discovery in standard locations
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	tempDir := t.TempDir()
	defer os.Chdir(originalWd)

	// Change to temp directory to isolate test
	os.Chdir(tempDir)

	testCases := []struct {
		name         string
		setupFiles   map[string]string // filename -> content
		expectedSize int
	}{
		{
			name: "config.json in current directory",
			setupFiles: map[string]string{
				"config.json": `{"fetchSize": 111}`,
			},
			expectedSize: 111,
		},
		{
			name: "bc-insights-tui.json in current directory",
			setupFiles: map[string]string{
				"bc-insights-tui.json": `{"fetchSize": 222}`,
			},
			expectedSize: 222,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up any existing files
			os.Remove("config.json")
			os.Remove("bc-insights-tui.json")

			// Create test files
			for filename, content := range tc.setupFiles {
				err := os.WriteFile(filename, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
				defer os.Remove(filename)
			}

			cfg := LoadConfigWithArgs([]string{})

			if cfg.LogFetchSize != tc.expectedSize {
				t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expectedSize, cfg.LogFetchSize)
			}
		})
	}
}

func TestLoadConfig_PriorityOrder(t *testing.T) {
	// Test precedence: flags > env > file > defaults
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "priority-test.json")

	// Create config file
	fileContent := `{
		"fetchSize": 100,
		"environment": "FileEnv",
		"applicationInsightsKey": "file-key"
	}`
	err := os.WriteFile(configFile, []byte(fileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Set environment variables
	os.Setenv("LOG_FETCH_SIZE", "200")
	os.Setenv("BCINSIGHTS_ENVIRONMENT", "EnvEnvironment")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	// Test: file < env < flags
	cfg := LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--fetch-size=300",
		"--environment=FlagEnv",
		// Note: not setting app-insights-key flag to test env wins over file
	})

	// Flag should win for fetchSize and environment
	if cfg.LogFetchSize != 300 {
		t.Errorf("Expected flags to override env and file for LogFetchSize, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "FlagEnv" {
		t.Errorf("Expected flags to override env and file for Environment, got %q", cfg.Environment)
	}
	// Env should win over file for applicationInsightsKey (no flag set)
	if cfg.ApplicationInsightsKey != "env-key" {
		t.Errorf("Expected env to override file for ApplicationInsightsKey, got %q", cfg.ApplicationInsightsKey)
	}
}

func TestConfig_ValidationRules(t *testing.T) {
	cfg := Config{}

	testCases := []struct {
		name        string
		setting     string
		value       string
		shouldError bool
		expectedMsg string
	}{
		// fetchSize validation
		{"valid fetchSize", "fetchSize", "100", false, ""},
		{"invalid fetchSize - negative", "fetchSize", "-10", true, "fetchSize must be a positive integer"},
		{"invalid fetchSize - zero", "fetchSize", "0", true, "fetchSize must be a positive integer"},
		{"invalid fetchSize - non-integer", "fetchSize", "abc", true, "fetchSize must be a positive integer"},
		{"invalid fetchSize - float", "fetchSize", "10.5", true, "fetchSize must be a positive integer"},

		// environment validation
		{"valid environment", "environment", "Production", false, ""},
		{"invalid environment - empty", "environment", "", true, "environment cannot be empty"},

		// applicationInsightsKey validation
		{"valid app insights key", "applicationInsightsKey", "InstrumentationKey=abc", false, ""},
		{"valid app insights key - empty", "applicationInsightsKey", "", false, ""}, // Allowed for clearing

		// unknown setting
		{"unknown setting", "unknownSetting", "value", true, "unknown setting: unknownSetting"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cfg.ValidateAndUpdateSetting(tc.setting, tc.value)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s=%s, but got none", tc.setting, tc.value)
				} else if !strings.Contains(err.Error(), tc.expectedMsg) {
					t.Errorf("Expected error containing %q, got %q", tc.expectedMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s=%s, but got: %v", tc.setting, tc.value, err)
				}
			}
		})
	}
}

func TestConfig_AllSettingsPresent(t *testing.T) {
	cfg := Config{
		LogFetchSize:           123,
		Environment:            "TestEnv",
		ApplicationInsightsKey: "test-key",
	}

	// Test all settings can be retrieved
	expectedSettings := map[string]bool{
		"fetchSize":              true,
		"environment":            true,
		"applicationInsightsKey": true,
	}

	settings := cfg.ListAllSettings()

	for expectedSetting := range expectedSettings {
		if _, exists := settings[expectedSetting]; !exists {
			t.Errorf("Expected setting %q to be present in ListAllSettings output", expectedSetting)
		}
	}

	// Test individual setting access
	for setting := range expectedSettings {
		_, err := cfg.GetSettingValue(setting)
		if err != nil {
			t.Errorf("Expected to be able to get value for setting %q, got error: %v", setting, err)
		}
	}

	// Test that we can validate and update all settings
	testValues := map[string]string{
		"fetchSize":              "456",
		"environment":            "NewEnv",
		"applicationInsightsKey": "new-key",
	}

	for setting, value := range testValues {
		err := cfg.ValidateAndUpdateSetting(setting, value)
		if err != nil {
			t.Errorf("Expected to be able to update setting %q with value %q, got error: %v", setting, value, err)
		}
	}
}

func TestConfig_JSONYAMLRoundtrip(t *testing.T) {
	originalCfg := Config{
		LogFetchSize:           99,
		Environment:            "RoundtripTest",
		ApplicationInsightsKey: "roundtrip-key",
	}

	// Test JSON serialization and deserialization
	t.Run("JSON roundtrip", func(t *testing.T) {
		// Serialize to JSON
		jsonData, err := json.Marshal(originalCfg)
		if err != nil {
			t.Fatalf("Failed to marshal config to JSON: %v", err)
		}

		// Deserialize from JSON
		var deserializedCfg Config
		err = json.Unmarshal(jsonData, &deserializedCfg)
		if err != nil {
			t.Fatalf("Failed to unmarshal config from JSON: %v", err)
		}

		// Compare
		if deserializedCfg.LogFetchSize != originalCfg.LogFetchSize {
			t.Errorf("JSON roundtrip failed for LogFetchSize: expected %d, got %d", originalCfg.LogFetchSize, deserializedCfg.LogFetchSize)
		}
		if deserializedCfg.Environment != originalCfg.Environment {
			t.Errorf("JSON roundtrip failed for Environment: expected %q, got %q", originalCfg.Environment, deserializedCfg.Environment)
		}
		if deserializedCfg.ApplicationInsightsKey != originalCfg.ApplicationInsightsKey {
			t.Errorf("JSON roundtrip failed for ApplicationInsightsKey: expected %q, got %q", originalCfg.ApplicationInsightsKey, deserializedCfg.ApplicationInsightsKey)
		}
	})

	// Test file-based roundtrip
	t.Run("File roundtrip", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "roundtrip-test.json")

		// Write config to file
		jsonData, _ := json.MarshalIndent(originalCfg, "", "  ")
		err := os.WriteFile(configFile, jsonData, 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Load config from file
		loadedCfg, err := loadConfigFromFile(configFile)
		if err != nil {
			t.Fatalf("Failed to load config from file: %v", err)
		}

		// Compare
		if loadedCfg.LogFetchSize != originalCfg.LogFetchSize {
			t.Errorf("File roundtrip failed for LogFetchSize: expected %d, got %d", originalCfg.LogFetchSize, loadedCfg.LogFetchSize)
		}
		if loadedCfg.Environment != originalCfg.Environment {
			t.Errorf("File roundtrip failed for Environment: expected %q, got %q", originalCfg.Environment, loadedCfg.Environment)
		}
		if loadedCfg.ApplicationInsightsKey != originalCfg.ApplicationInsightsKey {
			t.Errorf("File roundtrip failed for ApplicationInsightsKey: expected %q, got %q", originalCfg.ApplicationInsightsKey, loadedCfg.ApplicationInsightsKey)
		}
	})
}
