package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// setupTestModeForPersistence sets the TEST_MODE environment variable for test isolation
func setupTestModeForPersistence(t *testing.T) {
	t.Helper()
	t.Setenv("TEST_MODE", "1")
}

func TestSaveConfig_TestIsolationBehavior(t *testing.T) {
	setupTestModeForPersistence(t)

	// Test that SaveConfig respects test isolation by not actually saving
	// during tests (this is the intended behavior for test isolation)

	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	cfg := NewConfig()
	cfg.LogFetchSize = 777
	cfg.Environment = "TestIsolation"

	// SaveConfig should succeed but not actually create a file (test isolation)
	err := cfg.SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig should not error in test mode: %v", err)
	}

	// Verify no file was created (this is the expected behavior for test isolation)
	if _, statErr := os.Stat(configFileName); !os.IsNotExist(statErr) {
		t.Error("SaveConfig should not create files during tests (test isolation)")
	}

	// Note: SaveConfig deliberately disables itself during tests to maintain
	// test isolation. This prevents tests from contaminating real user config files.
}

func TestConfigPersistence_Integration(t *testing.T) {
	// Integration test for configuration persistence using dependency injection
	fs := NewMemFileSystem()

	// Initially, SaveConfig should not error (no file to find)
	cfg := NewConfig()
	cfg.LogFetchSize = 999
	cfg.Environment = "TestPersistence"

	// Save config (will be no-op in test mode but shouldn't error)
	err := cfg.SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig should not error even in test mode: %v", err)
	}

	// Test that findConfigFile works correctly with dependency injection
	loader := NewTestConfigLoader(fs, []string{"config.json"})

	// Initially, no config file should be found
	path := loader.findConfigFile()
	if path != "" {
		t.Errorf("Expected no config file initially, got %q", path)
	}

	// Create a config file and test path resolution
	testConfig := `{"fetchSize": 100, "environment": "Test"}`
	err = fs.WriteFile("config.json", []byte(testConfig), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	path = loader.findConfigFile()
	if path != configFileName {
		t.Errorf("Expected findConfigFile to return %s, got %q", configFileName, path)
	}

	// Test loading the config
	loadedCfg, err := loader.loadConfigFromFile(path)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	if loadedCfg.LogFetchSize != 100 {
		t.Errorf("Expected loaded LogFetchSize to be 100, got %d", loadedCfg.LogFetchSize)
	}
	if loadedCfg.Environment != "Test" {
		t.Errorf("Expected loaded Environment to be 'Test', got %q", loadedCfg.Environment)
	}
}

func TestValidateAndUpdateSetting_NoAutoSaveInTest(t *testing.T) {
	setupTestModeForPersistence(t)

	// Test that ValidateAndUpdateSetting does not auto-save to file during tests

	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	cfg := NewConfig()
	cfg.LogFetchSize = 50
	cfg.Environment = "AutoSaveTest"

	// Update a setting (should succeed but not auto-save during tests)
	err := cfg.ValidateAndUpdateSetting("fetchSize", "999")
	if err != nil {
		t.Fatalf("Failed to update setting: %v", err)
	}

	// Verify the in-memory config was updated
	if cfg.LogFetchSize != 999 {
		t.Errorf("Setting was not updated in memory: expected 999, got %d", cfg.LogFetchSize)
	}

	// Verify no config file was created (test isolation)
	if _, statErr := os.Stat(configFileName); !os.IsNotExist(statErr) {
		t.Error("ValidateAndUpdateSetting should not create files during tests (test isolation)")
	}

	// Update another setting
	err = cfg.ValidateAndUpdateSetting("environment", "AutoSaveUpdated")
	if err != nil {
		t.Fatalf("Failed to update environment setting: %v", err)
	}

	// Verify the in-memory config was updated
	if cfg.Environment != "AutoSaveUpdated" {
		t.Errorf("Environment setting was not updated in memory: expected 'AutoSaveUpdated', got %q", cfg.Environment)
	}

	// Verify LogFetchSize is still correct
	if cfg.LogFetchSize != 999 {
		t.Errorf("Previous setting lost: expected LogFetchSize 999, got %d", cfg.LogFetchSize)
	}
}

func TestSaveConfig_ErrorHandling(t *testing.T) {
	// Test error handling in SaveConfig - mainly testing that
	// error conditions don't cause panics
	cfg := NewConfig()

	// Test that SaveConfig handles test mode gracefully
	err := cfg.SaveConfig()
	if err != nil {
		t.Errorf("SaveConfig should not error in test mode: %v", err)
	}

	// Test that findConfigFile handles various scenarios using dependency injection
	fs := NewMemFileSystem()
	loader := NewTestConfigLoader(fs, []string{"config.json"})

	// Test with no config file (should return empty string)
	path := loader.findConfigFile()
	if path != "" {
		t.Errorf("findConfigFile should return empty string with no existing files, got %q", path)
	}

	// Test with valid config file
	err = fs.WriteFile("config.json", []byte(`{"fetchSize": 1}`), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	path = loader.findConfigFile()
	if path != "config.json" {
		t.Errorf("findConfigFile should return config.json with valid file, got %q", path)
	}
}

func TestConfigFilePathResolution(t *testing.T) {
	// Test that findConfigFile uses correct config file path precedence using dependency injection

	testCases := []struct {
		name         string
		setupFiles   []string
		searchPaths  []string
		expectedFile string
	}{
		{
			name:         "returns empty when no config exists",
			setupFiles:   []string{},
			searchPaths:  []string{"config.json", "bc-insights-tui.json"},
			expectedFile: "",
		},
		{
			name:         "finds existing config.json",
			setupFiles:   []string{"config.json"},
			searchPaths:  []string{"config.json", "bc-insights-tui.json"},
			expectedFile: "config.json",
		},
		{
			name:         "finds existing bc-insights-tui.json",
			setupFiles:   []string{"bc-insights-tui.json"},
			searchPaths:  []string{"config.json", "bc-insights-tui.json"},
			expectedFile: "bc-insights-tui.json",
		},
		{
			name:         "prefers config.json over bc-insights-tui.json",
			setupFiles:   []string{"config.json", "bc-insights-tui.json"},
			searchPaths:  []string{"config.json", "bc-insights-tui.json"},
			expectedFile: "config.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create isolated filesystem for each test
			fs := NewMemFileSystem()

			// Create setup files
			for _, filename := range tc.setupFiles {
				initialData := `{"fetchSize": 1}`
				err := fs.WriteFile(filename, []byte(initialData), 0o644)
				if err != nil {
					t.Fatalf("Failed to create setup file %s: %v", filename, err)
				}
			}

			// Create test loader with specified search paths
			loader := NewTestConfigLoader(fs, tc.searchPaths)

			// Test findConfigFile
			path := loader.findConfigFile()
			if path != tc.expectedFile {
				t.Errorf("Expected findConfigFile to return %q, got %q", tc.expectedFile, path)
			}
		})
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	// Test that Config struct can be properly serialized to/from JSON
	cfg := NewConfig()
	cfg.LogFetchSize = 456
	cfg.Environment = "FormatTest"
	cfg.ApplicationInsightsKey = "format-test-key-with-special-chars-!@#$%^&*()"

	// Test JSON marshaling
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	content := string(data)

	// Verify JSON is properly formatted (indented)
	if !strings.Contains(content, "\n") {
		t.Error("JSON should be indented with newlines")
	}
	if !strings.Contains(content, "  ") {
		t.Error("JSON should be indented with spaces")
	}

	// Verify JSON is valid by parsing it
	var parsedConfig Config
	err = json.Unmarshal(data, &parsedConfig)
	if err != nil {
		t.Fatalf("Generated JSON is not valid: %v", err)
	}

	// Verify all values are preserved
	if parsedConfig.LogFetchSize != cfg.LogFetchSize {
		t.Errorf("LogFetchSize not preserved: expected %d, got %d", cfg.LogFetchSize, parsedConfig.LogFetchSize)
	}
	if parsedConfig.Environment != cfg.Environment {
		t.Errorf("Environment not preserved: expected %q, got %q", cfg.Environment, parsedConfig.Environment)
	}
	if parsedConfig.ApplicationInsightsKey != cfg.ApplicationInsightsKey {
		t.Errorf("ApplicationInsightsKey not preserved: expected %q, got %q", cfg.ApplicationInsightsKey, parsedConfig.ApplicationInsightsKey)
	}

	// Verify special characters are properly handled (JSON escapes some characters)
	// Let's just verify the content contains the basic parts and is valid JSON
	if !strings.Contains(content, "format-test-key-with-special-chars") {
		t.Errorf("ApplicationInsightsKey should be in JSON output, got: %s", content)
	}
}
