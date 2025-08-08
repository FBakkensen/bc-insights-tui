package config

import (
	"encoding/json"
	"testing"
)

const (
	testFlagEnv = "FlagEnv"
	testEnvKey  = "env-key"
)

// createTestLoader creates a ConfigLoader with in-memory filesystem for testing
func createTestLoader(t *testing.T) (*ConfigLoader, *MemFileSystem) {
	t.Helper()
	fs := NewMemFileSystem()
	loader := NewTestConfigLoader(fs, []string{"/test/config.json", "/home/user/.config/bc-insights-tui/config.json"})
	return loader, fs
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	loader, _ := createTestLoader(t)

	cfg := loader.LoadWithArgs([]string{})

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
			loader, _ := createTestLoader(t)

			// Set environment variable using test helper
			t.Setenv("LOG_FETCH_SIZE", tc.envValue)

			cfg := loader.LoadWithArgs([]string{})

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
			loader, _ := createTestLoader(t)

			// Set environment variable using test helper
			t.Setenv("LOG_FETCH_SIZE", tc.envValue)

			cfg := loader.LoadWithArgs([]string{})

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
			loader, _ := createTestLoader(t)

			// Set environment variable using test helper
			t.Setenv("LOG_FETCH_SIZE", tc.envValue)

			cfg := loader.LoadWithArgs([]string{})

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

// createTestLoaderWithFlags creates a ConfigLoader with in-memory filesystem and mock flags for testing
func createTestLoaderWithFlags(t *testing.T, flags map[string]interface{}) (*ConfigLoader, *MemFileSystem) {
	t.Helper()
	fs := NewMemFileSystem()
	loader := NewTestConfigLoader(fs, []string{"/test/config.json", "/home/user/.config/bc-insights-tui/config.json"})

	// Set up mock flags if provided
	if len(flags) > 0 {
		parsedFlags := &ParsedFlags{}

		if val, ok := flags["fetch-size"]; ok {
			if intVal, ok := val.(int); ok {
				parsedFlags.fetchSize = &intVal
			}
		}
		if val, ok := flags["environment"]; ok {
			if strVal, ok := val.(string); ok {
				parsedFlags.environment = &strVal
			}
		}
		if val, ok := flags["app-insights-key"]; ok {
			if strVal, ok := val.(string); ok {
				parsedFlags.applicationInsights = &strVal
			}
		}

		if mockParser, ok := loader.flagParser.(*MockFlagParser); ok {
			mockParser.SetFlags(parsedFlags)
		}
	}

	return loader, fs
}

func TestLoadConfig_CommandLineFlags(t *testing.T) {
	// Test command line flags override environment variables
	testCases := []struct {
		name     string
		flags    map[string]interface{}
		envVars  map[string]string
		expected Config
	}{
		{
			name:    "fetch-size flag overrides env",
			flags:   map[string]interface{}{"fetch-size": 200},
			envVars: map[string]string{"LOG_FETCH_SIZE": "100"},
			expected: Config{
				LogFetchSize:           200,
				Environment:            "Development",
				ApplicationInsightsKey: "",
			},
		},
		{
			name:    "environment flag overrides env",
			flags:   map[string]interface{}{"environment": "Testing"},
			envVars: map[string]string{"BCINSIGHTS_ENVIRONMENT": "Production"},
			expected: Config{
				LogFetchSize:           50,
				Environment:            "Testing",
				ApplicationInsightsKey: "",
			},
		},
		{
			name:    "app-insights-key flag overrides env",
			flags:   map[string]interface{}{"app-insights-key": "flag-key"},
			envVars: map[string]string{"BCINSIGHTS_APP_INSIGHTS_KEY": testEnvKey},
			expected: Config{
				LogFetchSize:           50,
				Environment:            "Development",
				ApplicationInsightsKey: "flag-key",
			},
		},
		{
			name: "multiple flags override multiple env vars",
			flags: map[string]interface{}{
				"fetch-size":       300,
				"environment":      "Staging",
				"app-insights-key": "multi-key",
			},
			envVars: map[string]string{
				"LOG_FETCH_SIZE":              "100",
				"BCINSIGHTS_ENVIRONMENT":      "Production",
				"BCINSIGHTS_APP_INSIGHTS_KEY": testEnvKey,
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
			loader, _ := createTestLoaderWithFlags(t, tc.flags)

			// Set environment variables using test helper
			for key, value := range tc.envVars {
				t.Setenv(key, value)
			}

			cfg := loader.LoadWithArgs([]string{})

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
			_, fs := createTestLoader(t)

			// Create test config file in memory filesystem
			configPath := "/test/" + tc.fileName
			fs.WriteFile(configPath, []byte(tc.fileContent), 0o644)

			// Create loader that searches this path
			loader := NewTestConfigLoader(fs, []string{configPath})
			cfg := loader.LoadWithArgs([]string{})

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
	testCases := []struct {
		name         string
		setupFiles   map[string]string // filepath -> content
		expectedSize int
	}{
		{
			name: "config.json in current directory",
			setupFiles: map[string]string{
				"/test/config.json": `{"fetchSize": 111}`,
			},
			expectedSize: 111,
		},
		{
			name: "config.json in home directory",
			setupFiles: map[string]string{
				"/home/user/.config/bc-insights-tui/config.json": `{"fetchSize": 222}`,
			},
			expectedSize: 222,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loader, fs := createTestLoader(t)

			// Create test files in memory filesystem
			for filepath, content := range tc.setupFiles {
				fs.WriteFile(filepath, []byte(content), 0o644)
			}

			cfg := loader.LoadWithArgs([]string{})

			if cfg.LogFetchSize != tc.expectedSize {
				t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expectedSize, cfg.LogFetchSize)
			}
		})
	}
}

func TestLoadConfig_PriorityOrder(t *testing.T) {
	// Test precedence: flags > env > file > defaults
	loader, fs := createTestLoader(t)

	// Create config file in memory filesystem
	fileContent := `{
		"fetchSize": 100,
		"environment": "FileEnv",
		"applicationInsightsKey": "file-key"
	}`
	fs.WriteFile("/test/config.json", []byte(fileContent), 0o644)

	// Set environment variables using test helper
	t.Setenv("LOG_FETCH_SIZE", "200")
	t.Setenv("BCINSIGHTS_ENVIRONMENT", "EnvEnvironment")
	t.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", testEnvKey)

	// Set up mock flags to override some values
	mockFlags := &ParsedFlags{
		fetchSize:   intPtr(300),
		environment: stringPtr(testFlagEnv),
		// Note: not setting app-insights-key flag to test env wins over file
	}
	if mockParser, ok := loader.flagParser.(*MockFlagParser); ok {
		mockParser.SetFlags(mockFlags)
	}

	cfg := loader.LoadWithArgs([]string{})

	// Flag should win for fetchSize and environment
	if cfg.LogFetchSize != 300 {
		t.Errorf("Expected flags to override env and file for LogFetchSize, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != testFlagEnv {
		t.Errorf("Expected flags to override env and file for Environment, got %q", cfg.Environment)
	}
	// Env should win over file for applicationInsightsKey (no flag set)
	if cfg.ApplicationInsightsKey != testEnvKey {
		t.Errorf("Expected env to override file for ApplicationInsightsKey, got %q", cfg.ApplicationInsightsKey)
	}
}

func TestConfig_ValidationRules(t *testing.T) {
	cfg := NewConfig()

	testCases := []struct {
		name        string
		setting     string
		value       string
		shouldError bool
		expectedMsg string
	}{
		// fetchSize validation
		{"valid fetchSize", "fetchSize", "100", false, ""},
		{"invalid fetchSize - negative", "fetchSize", "-10", true, "fetchSize must be a positive integer, got: -10"},
		{"invalid fetchSize - zero", "fetchSize", "0", true, "fetchSize must be a positive integer, got: 0"},
		{"invalid fetchSize - non-integer", "fetchSize", "abc", true, "fetchSize must be a positive integer, got: abc"},
		{"invalid fetchSize - float", "fetchSize", "10.5", true, "fetchSize must be a positive integer, got: 10.5"},

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
				} else if tc.expectedMsg != "" {
					// Check error message if specified
					if err.Error() != tc.expectedMsg {
						t.Errorf("Expected error %q, got %q", tc.expectedMsg, err.Error())
					}
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
	cfg := NewConfig()
	cfg.LogFetchSize = 123
	cfg.Environment = "TestEnv"
	cfg.ApplicationInsightsKey = "test-key"

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

	// Test file-based roundtrip using in-memory filesystem
	t.Run("File roundtrip", func(t *testing.T) {
		_, fs := createTestLoader(t)

		configPath := "/test/roundtrip-test.json"

		// Write config to in-memory file
		jsonData, _ := json.MarshalIndent(originalCfg, "", "  ")
		fs.WriteFile(configPath, jsonData, 0o644)

		// Load config from in-memory file
		loader := NewTestConfigLoader(fs, []string{configPath})
		loadedCfg := loader.LoadWithArgs([]string{})

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
