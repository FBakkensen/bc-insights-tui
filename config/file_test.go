package config

import (
	"encoding/json"
	"testing"
)

// Test config file loading, parsing, and discovery functionality

func TestFindConfigFile_CurrentDirectory(t *testing.T) {
	// Test config file discovery in current directory using dependency injection
	testCases := []struct {
		name         string
		createFiles  []string
		expectedFile string
	}{
		{
			name:         "config.json found",
			createFiles:  []string{"config.json"},
			expectedFile: "config.json",
		},
		{
			name:         "bc-insights-tui.json found",
			createFiles:  []string{"bc-insights-tui.json"},
			expectedFile: "bc-insights-tui.json",
		},
		{
			name:         "config.json preferred over bc-insights-tui.json",
			createFiles:  []string{"config.json", "bc-insights-tui.json"},
			expectedFile: "config.json",
		},
		{
			name:         "no config files found",
			createFiles:  []string{},
			expectedFile: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create isolated filesystem for each test
			fs := NewMemFileSystem()

			// Create test files
			for _, filename := range tc.createFiles {
				err := fs.WriteFile(filename, []byte(`{"fetchSize": 1}`), 0o644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			// Define search paths for current directory
			searchPaths := []string{"config.json", "bc-insights-tui.json"}

			// Create test loader with isolated filesystem
			loader := NewTestConfigLoader(fs, searchPaths)

			result := loader.findConfigFile()
			if result != tc.expectedFile {
				t.Errorf("Expected findConfigFile to return %q, got %q", tc.expectedFile, result)
			}
		})
	}
}

func TestFindConfigFile_HomeDirectory(t *testing.T) {
	// Test config file discovery in home directory using dependency injection
	testCases := []struct {
		name         string
		createFile   string
		expectedFile string
	}{
		{
			name:         "config in .bc-insights-tui directory",
			createFile:   "/home/.bc-insights-tui/config.json",
			expectedFile: "/home/.bc-insights-tui/config.json",
		},
		{
			name:         "config in home directory",
			createFile:   "/home/.bc-insights-tui.json",
			expectedFile: "/home/.bc-insights-tui.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create isolated filesystem
			fs := NewMemFileSystem()

			// Create test file
			err := fs.WriteFile(tc.createFile, []byte(`{"fetchSize": 1}`), 0o644)
			if err != nil {
				t.Fatalf("Failed to create test file %s: %v", tc.createFile, err)
			}

			// Define search paths for home directory config files
			searchPaths := []string{tc.createFile}

			// Create test loader with isolated filesystem
			loader := NewTestConfigLoader(fs, searchPaths)

			result := loader.findConfigFile()
			if result != tc.expectedFile {
				t.Errorf("Expected findConfigFile to return %q, got %q", tc.expectedFile, result)
			}
		})
	}
}

func TestLoadConfigFromFile_ValidFiles(t *testing.T) {
	// Create isolated filesystem
	fs := NewMemFileSystem()

	testCases := []struct {
		name        string
		content     string
		expected    Config
		shouldError bool
	}{
		{
			name: "complete config",
			content: `{
				"fetchSize": 150,
				"environment": "TestComplete",
				"applicationInsightsKey": "complete-key"
			}`,
			expected: Config{
				LogFetchSize:           150,
				Environment:            "TestComplete",
				ApplicationInsightsKey: "complete-key",
			},
			shouldError: false,
		},
		{
			name: "partial config - only fetchSize",
			content: `{
				"fetchSize": 75
			}`,
			expected: Config{
				LogFetchSize:           75,
				Environment:            "",
				ApplicationInsightsKey: "",
			},
			shouldError: false,
		},
		{
			name: "partial config - only environment",
			content: `{
				"environment": "PartialEnv"
			}`,
			expected: Config{
				LogFetchSize:           0,
				Environment:            "PartialEnv",
				ApplicationInsightsKey: "",
			},
			shouldError: false,
		},
		{
			name:        "empty JSON object",
			content:     `{}`,
			expected:    Config{},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filename := "/test/" + tc.name + ".json"
			err := fs.WriteFile(filename, []byte(tc.content), 0o644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create test loader and load config from file
			loader := NewTestConfigLoader(fs, []string{})
			config, err := loader.loadConfigFromFile(filename)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error loading config from %s, got none", tc.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error loading config from %s, got: %v", tc.name, err)
				return
			}

			if config.LogFetchSize != tc.expected.LogFetchSize {
				t.Errorf("Expected LogFetchSize %d, got %d", tc.expected.LogFetchSize, config.LogFetchSize)
			}
			if config.Environment != tc.expected.Environment {
				t.Errorf("Expected Environment %q, got %q", tc.expected.Environment, config.Environment)
			}
			if config.ApplicationInsightsKey != tc.expected.ApplicationInsightsKey {
				t.Errorf("Expected ApplicationInsightsKey %q, got %q", tc.expected.ApplicationInsightsKey, config.ApplicationInsightsKey)
			}
		})
	}
}

func TestLoadConfigFromFile_InvalidFiles(t *testing.T) {
	// Create isolated filesystem
	fs := NewMemFileSystem()

	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "invalid JSON - syntax error",
			content: `{"fetchSize": 100,}`, // Trailing comma
		},
		{
			name:    "invalid JSON - malformed",
			content: `{fetchSize: 100}`, // Missing quotes
		},
		{
			name:    "not JSON at all",
			content: `This is not JSON`,
		},
		{
			name:    "empty file",
			content: ``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filename := "/test/" + tc.name + ".json"
			err := fs.WriteFile(filename, []byte(tc.content), 0o644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create test loader and load config from file
			loader := NewTestConfigLoader(fs, []string{})
			_, err = loader.loadConfigFromFile(filename)
			if err == nil {
				t.Errorf("Expected error loading invalid config file %s, got none", tc.name)
			}
		})
	}
}

func TestLoadConfigFromFile_FileNotFound(t *testing.T) {
	// Create isolated filesystem
	fs := NewMemFileSystem()

	nonExistentFile := "/path/that/does/not/exist/config.json"

	// Create test loader
	loader := NewTestConfigLoader(fs, []string{})
	_, err := loader.loadConfigFromFile(nonExistentFile)
	if err == nil {
		t.Error("Expected error when loading non-existent file, got none")
	}
}

func TestMergeConfig(t *testing.T) {
	testCases := []struct {
		name     string
		base     Config
		file     Config
		expected Config
	}{
		{
			name: "file overrides all base values",
			base: Config{
				LogFetchSize:           50,
				Environment:            "BaseEnv",
				ApplicationInsightsKey: "base-key",
			},
			file: Config{
				LogFetchSize:           100,
				Environment:            "FileEnv",
				ApplicationInsightsKey: "file-key",
			},
			expected: Config{
				LogFetchSize:           100,
				Environment:            "FileEnv",
				ApplicationInsightsKey: "file-key",
			},
		},
		{
			name: "file provides partial override",
			base: Config{
				LogFetchSize:           50,
				Environment:            "BaseEnv",
				ApplicationInsightsKey: "base-key",
			},
			file: Config{
				LogFetchSize: 200,
				Environment:  "FileEnv",
				// ApplicationInsightsKey not set in file
			},
			expected: Config{
				LogFetchSize:           200,
				Environment:            "FileEnv",
				ApplicationInsightsKey: "base-key", // Preserved from base
			},
		},
		{
			name: "file has zero values (should not override)",
			base: Config{
				LogFetchSize:           50,
				Environment:            "BaseEnv",
				ApplicationInsightsKey: "base-key",
			},
			file: Config{
				LogFetchSize: 0,  // Zero value, should not override
				Environment:  "", // Empty string, should not override
				// ApplicationInsightsKey not set
			},
			expected: Config{
				LogFetchSize:           50,         // Preserved from base
				Environment:            "BaseEnv",  // Preserved from base
				ApplicationInsightsKey: "base-key", // Preserved from base
			},
		},
		{
			name: "empty file config",
			base: Config{
				LogFetchSize:           50,
				Environment:            "BaseEnv",
				ApplicationInsightsKey: "base-key",
			},
			file: Config{}, // All zero values
			expected: Config{
				LogFetchSize:           50, // All preserved from base
				Environment:            "BaseEnv",
				ApplicationInsightsKey: "base-key",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			base := tc.base // Make a copy
			mergeConfig(&base, &tc.file)

			if base.LogFetchSize != tc.expected.LogFetchSize {
				t.Errorf("Expected LogFetchSize %d, got %d", tc.expected.LogFetchSize, base.LogFetchSize)
			}
			if base.Environment != tc.expected.Environment {
				t.Errorf("Expected Environment %q, got %q", tc.expected.Environment, base.Environment)
			}
			if base.ApplicationInsightsKey != tc.expected.ApplicationInsightsKey {
				t.Errorf("Expected ApplicationInsightsKey %q, got %q", tc.expected.ApplicationInsightsKey, base.ApplicationInsightsKey)
			}
		})
	}
}

func TestConfigFileIntegration(t *testing.T) {
	// Test the complete config file integration with LoadConfig using dependency injection
	fs := NewMemFileSystem()
	configFile := "/test/integration.json"

	// Create config file
	configData := Config{
		LogFetchSize:           333,
		Environment:            "FileIntegration",
		ApplicationInsightsKey: "integration-key-123456789",
	}
	jsonData, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = fs.WriteFile(configFile, jsonData, 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create test loader and configure it to use our specific config file
	searchPaths := []string{configFile}
	loader := NewTestConfigLoader(fs, searchPaths)

	// Load config using the test loader
	cfg := loader.LoadWithArgs([]string{})

	// Verify file values are loaded
	if cfg.LogFetchSize != 333 {
		t.Errorf("Expected LogFetchSize from file to be 333, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "FileIntegration" {
		t.Errorf("Expected Environment from file to be 'FileIntegration', got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "integration-key-123456789" {
		t.Errorf("Expected ApplicationInsightsKey from file, got %q", cfg.ApplicationInsightsKey)
	}
}
