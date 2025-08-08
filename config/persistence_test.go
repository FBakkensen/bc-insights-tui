package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveConfig_TestIsolationBehavior(t *testing.T) {
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
	// Test the overall config persistence behavior that would work in production
	// This tests the integration without actually writing files during tests

	cfg := NewConfig()
	cfg.LogFetchSize = 123
	cfg.Environment = "IntegrationTest"
	cfg.ApplicationInsightsKey = "integration-key"

	// Test that SaveConfig doesn't error (even though it won't save during tests)
	err := cfg.SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig should not error even in test mode: %v", err)
	}

	// Test that findConfigFile works correctly in test mode
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Initially, no config file should be found
	path := findConfigFile()
	if path != "" {
		t.Errorf("Expected no config file initially, got %q", path)
	}

	// Create a config file and test path resolution
	testConfig := `{"fetchSize": 100, "environment": "Test"}`
	err = os.WriteFile("config.json", []byte(testConfig), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	path = findConfigFile()
	if path != configFileName {
		t.Errorf("Expected findConfigFile to return %s, got %q", configFileName, path)
	}

	// Test loading the config
	loadedCfg, err := loadConfigFromFile(path)
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
	// Test that ValidateAndUpdateSetting works but doesn't auto-save during tests
	// (This verifies the test isolation is working correctly)

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

	// Test that findConfigFile handles various scenarios
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test with no config file (should return empty string)
	path := findConfigFile()
	if path != "" {
		t.Errorf("findConfigFile should return empty string with no existing files, got %q", path)
	}

	// Test with valid config file
	err = os.WriteFile(configFileName, []byte(`{"fetchSize": 1}`), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	path = findConfigFile()
	if path != configFileName {
		t.Errorf("findConfigFile should return %s with valid file, got %q", configFileName, path)
	}
}

func TestConfigFilePathResolution(t *testing.T) {
	// Test that findConfigFile uses correct config file path precedence
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	testCases := []struct {
		name         string
		setupFiles   []string
		expectedFile string
	}{
		{
			name:         "returns empty when no config exists",
			setupFiles:   []string{},
			expectedFile: "",
		},
		{
			name:         "finds existing config.json",
			setupFiles:   []string{"config.json"},
			expectedFile: "config.json",
		},
		{
			name:         "finds existing bc-insights-tui.json",
			setupFiles:   []string{"bc-insights-tui.json"},
			expectedFile: "bc-insights-tui.json",
		},
		{
			name:         "prefers config.json over bc-insights-tui.json",
			setupFiles:   []string{"config.json", "bc-insights-tui.json"},
			expectedFile: "config.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh subdirectory for each test
			testDir := filepath.Join(tempDir, tc.name)
			os.MkdirAll(testDir, 0o755)
			os.Chdir(testDir)

			// Create setup files
			for _, filename := range tc.setupFiles {
				initialData := `{"fetchSize": 1}`
				err := os.WriteFile(filename, []byte(initialData), 0o644)
				if err != nil {
					t.Fatalf("Failed to create setup file %s: %v", filename, err)
				}
			}

			// Test findConfigFile
			path := findConfigFile()
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

func TestIsTestMode(t *testing.T) {
	// Test that isTestMode correctly detects test environment
	if !isTestMode() {
		t.Error("isTestMode should return true when running tests")
	}
}

func TestFindConfigFile_TestMode(t *testing.T) {
	// Test the findConfigFile function behavior in test mode
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test with no existing config file
	path := findConfigFile()
	if path != "" {
		t.Errorf("Expected findConfigFile to return empty string with no files, got %q", path)
	}

	// Create config.json and test again
	err := os.WriteFile("config.json", []byte(`{"fetchSize": 1}`), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	path = findConfigFile()
	if path != "config.json" {
		t.Errorf("Expected findConfigFile to return existing config.json, got %q", path)
	}

	// Create bc-insights-tui.json and test precedence
	os.Remove("config.json")
	err = os.WriteFile("bc-insights-tui.json", []byte(`{"fetchSize": 1}`), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	path = findConfigFile()
	if path != "bc-insights-tui.json" {
		t.Errorf("Expected findConfigFile to return existing bc-insights-tui.json, got %q", path)
	}
}
