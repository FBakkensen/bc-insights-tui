package config

import (
	"testing"
)

// TestConfigLoader_DependencyInjection demonstrates the new clean approach
// to config loading that completely isolates tests from the real filesystem.
func TestConfigLoader_DependencyInjection(t *testing.T) {
	t.Run("loads config from in-memory filesystem", func(t *testing.T) {
		// Create in-memory filesystem
		fs := NewMemFileSystem()

		// Set up test config file
		configContent := `{
			"fetchSize": 500,
			"environment": "test-env",
			"applicationInsightsKey": "test-key-123"
		}`
		fs.WriteFile("/test/config.json", []byte(configContent), 0o644)

		// Create test loader with controlled dependencies
		loader := NewTestConfigLoader(fs, []string{"/test/config.json"})

		// Load config - completely isolated from real filesystem
		cfg := loader.LoadWithArgs([]string{})

		// Verify config loaded correctly
		if cfg.LogFetchSize != 500 {
			t.Errorf("Expected LogFetchSize 500, got %d", cfg.LogFetchSize)
		}
		if cfg.Environment != "test-env" {
			t.Errorf("Expected Environment 'test-env', got %s", cfg.Environment)
		}
		if cfg.ApplicationInsightsKey != "test-key-123" {
			t.Errorf("Expected ApplicationInsightsKey 'test-key-123', got %s", cfg.ApplicationInsightsKey)
		}
	})

	t.Run("handles missing config file gracefully", func(t *testing.T) {
		// Create empty in-memory filesystem
		fs := NewMemFileSystem()

		// Create test loader looking for non-existent file
		loader := NewTestConfigLoader(fs, []string{"/missing/config.json"})

		// Load config - should get defaults
		cfg := loader.LoadWithArgs([]string{})

		// Should get default values
		if cfg.LogFetchSize != 50 {
			t.Errorf("Expected default LogFetchSize %d, got %d", 50, cfg.LogFetchSize)
		}
		if cfg.Environment != "Development" {
			t.Errorf("Expected default Environment %s, got %s", "Development", cfg.Environment)
		}
	})

	t.Run("searches multiple paths in order", func(t *testing.T) {
		// Create in-memory filesystem
		fs := NewMemFileSystem()

		// Set up config files with different values
		fs.WriteFile("/first/config.json", []byte(`{"fetchSize": 100}`), 0o644)
		fs.WriteFile("/second/config.json", []byte(`{"fetchSize": 200}`), 0o644)

		// Create test loader with multiple search paths
		loader := NewTestConfigLoader(fs, []string{
			"/missing/config.json", // doesn't exist
			"/first/config.json",   // should find this one
			"/second/config.json",  // should not reach this one
		})

		// Load config
		cfg := loader.LoadWithArgs([]string{})

		// Should find the first existing file
		if cfg.LogFetchSize != 100 {
			t.Errorf("Expected LogFetchSize 100, got %d", cfg.LogFetchSize)
		}
	})

	t.Run("command line flags override file config", func(t *testing.T) {
		// Create in-memory filesystem
		fs := NewMemFileSystem()

		// Set up config file
		fs.WriteFile("/test/config.json", []byte(`{"fetchSize": 100}`), 0o644)

		// Create test loader
		loader := NewTestConfigLoader(fs, []string{"/test/config.json"})

		// Set mock flags to override file value
		mockFlags := &ParsedFlags{
			fetchSize: intPtr(500),
		}
		if mockParser, ok := loader.flagParser.(*MockFlagParser); ok {
			mockParser.SetFlags(mockFlags)
		}

		// Load config with flag override
		cfg := loader.LoadWithArgs([]string{})

		// Flag should override file value
		if cfg.LogFetchSize != 500 {
			t.Errorf("Expected LogFetchSize 500 (flag override), got %d", cfg.LogFetchSize)
		}
	})
}

// intPtr returns a pointer to an int for testing
func intPtr(i int) *int {
	return &i
}

// stringPtr returns a pointer to a string for testing
func stringPtr(s string) *string {
	return &s
}
