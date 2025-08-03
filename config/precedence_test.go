package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test priority order between config sources: flags > env > file > defaults

func TestPrecedence_DefaultsOnly(t *testing.T) {
	// Test that defaults are used when no other sources are available

	// Ensure clean environment
	envVars := []string{"LOG_FETCH_SIZE", "BCINSIGHTS_ENVIRONMENT", "BCINSIGHTS_APP_INSIGHTS_KEY"}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	cfg := LoadConfigWithArgs([]string{})

	// Should get default values
	expected := Config{
		LogFetchSize:           50,
		Environment:            "Development",
		ApplicationInsightsKey: "",
	}

	if cfg.LogFetchSize != expected.LogFetchSize {
		t.Errorf("Expected default LogFetchSize %d, got %d", expected.LogFetchSize, cfg.LogFetchSize)
	}
	if cfg.Environment != expected.Environment {
		t.Errorf("Expected default Environment %q, got %q", expected.Environment, cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != expected.ApplicationInsightsKey {
		t.Errorf("Expected default ApplicationInsightsKey %q, got %q", expected.ApplicationInsightsKey, cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_FileOverridesDefaults(t *testing.T) {
	// Test that config file values override defaults
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "precedence-file.json")

	// Create config file with different values than defaults
	fileConfig := Config{
		LogFetchSize:           111,
		Environment:            "FileEnv",
		ApplicationInsightsKey: "file-key",
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Ensure clean environment
	os.Unsetenv("LOG_FETCH_SIZE")
	os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
	os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")

	cfg := LoadConfigWithArgs([]string{"--config=" + configFile})

	// Should get file values, not defaults
	if cfg.LogFetchSize != 111 {
		t.Errorf("Expected file LogFetchSize 111, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "FileEnv" {
		t.Errorf("Expected file Environment 'FileEnv', got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "file-key" {
		t.Errorf("Expected file ApplicationInsightsKey 'file-key', got %q", cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_EnvOverridesFileAndDefaults(t *testing.T) {
	// Test that environment variables override file and defaults
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "precedence-env.json")

	// Create config file
	fileConfig := Config{
		LogFetchSize:           111,
		Environment:            "FileEnv",
		ApplicationInsightsKey: "file-key",
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Set environment variables with different values
	os.Setenv("LOG_FETCH_SIZE", "222")
	os.Setenv("BCINSIGHTS_ENVIRONMENT", "EnvEnvironment")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	cfg := LoadConfigWithArgs([]string{"--config=" + configFile})

	// Should get env values, not file or defaults
	if cfg.LogFetchSize != 222 {
		t.Errorf("Expected env LogFetchSize 222, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "EnvEnvironment" {
		t.Errorf("Expected env Environment 'EnvEnvironment', got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "env-key" {
		t.Errorf("Expected env ApplicationInsightsKey 'env-key', got %q", cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_FlagsOverrideAll(t *testing.T) {
	// Test that command line flags override env, file, and defaults
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "precedence-flags.json")

	// Create config file
	fileConfig := Config{
		LogFetchSize:           111,
		Environment:            "FileEnv",
		ApplicationInsightsKey: "file-key",
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Set environment variables
	os.Setenv("LOG_FETCH_SIZE", "222")
	os.Setenv("BCINSIGHTS_ENVIRONMENT", "EnvEnvironment")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	// Load with command line flags
	cfg := LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--fetch-size=333",
		"--environment=FlagEnv",
		"--app-insights-key=flag-key",
	})

	// Should get flag values, overriding env, file, and defaults
	if cfg.LogFetchSize != 333 {
		t.Errorf("Expected flag LogFetchSize 333, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "FlagEnv" {
		t.Errorf("Expected flag Environment 'FlagEnv', got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "flag-key" {
		t.Errorf("Expected flag ApplicationInsightsKey 'flag-key', got %q", cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_PartialOverrides(t *testing.T) {
	// Test that partial overrides work correctly - each source only overrides what it specifies
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "precedence-partial.json")

	// Create config file with only some values
	fileConfig := Config{
		LogFetchSize: 111,
		Environment:  "FileEnv",
		// ApplicationInsightsKey not set in file
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Set only some environment variables
	os.Setenv("LOG_FETCH_SIZE", "222")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key")
	// BCINSIGHTS_ENVIRONMENT not set
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	// Set only some flags
	cfg := LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--environment=FlagEnv",
		// --fetch-size and --app-insights-key not set
	})

	// Expected result:
	// - LogFetchSize: env (222) wins over file (111)
	// - Environment: flag (FlagEnv) wins over file (FileEnv)
	// - ApplicationInsightsKey: env (env-key) wins over default ("")
	if cfg.LogFetchSize != 222 {
		t.Errorf("Expected env to win for LogFetchSize, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "FlagEnv" {
		t.Errorf("Expected flag to win for Environment, got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "env-key" {
		t.Errorf("Expected env to win for ApplicationInsightsKey, got %q", cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_InvalidEnvVarsFallback(t *testing.T) {
	// Test that invalid environment variables fall back to next priority level
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "precedence-invalid.json")

	// Create config file
	fileConfig := Config{
		LogFetchSize:           111,
		Environment:            "FileEnv",
		ApplicationInsightsKey: "file-key",
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Set invalid environment variables
	os.Setenv("LOG_FETCH_SIZE", "invalid-not-a-number")
	os.Setenv("BCINSIGHTS_ENVIRONMENT", "") // Empty string, but this is valid for env
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "valid-env-key")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	cfg := LoadConfigWithArgs([]string{"--config=" + configFile})

	// Expected result:
	// - LogFetchSize: invalid env var should fall back to file (111)
	// - Environment: empty env var should be used (overrides file)
	// - ApplicationInsightsKey: valid env var should be used
	if cfg.LogFetchSize != 111 {
		t.Errorf("Expected fallback to file for invalid env LogFetchSize, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "" {
		t.Errorf("Expected empty env Environment to override file, got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "valid-env-key" {
		t.Errorf("Expected valid env ApplicationInsightsKey, got %q", cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_ZeroValueFlags(t *testing.T) {
	// Test behavior with zero-value flags
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "precedence-zero.json")

	// Create config file
	fileConfig := Config{
		LogFetchSize:           111,
		Environment:            "FileEnv",
		ApplicationInsightsKey: "file-key",
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Set environment variables
	os.Setenv("LOG_FETCH_SIZE", "222")
	os.Setenv("BCINSIGHTS_ENVIRONMENT", "EnvEnvironment")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	// Load with zero-value flags (should not override)
	cfg := LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--fetch-size=0", // Zero value, should not override
		"--environment=", // Empty string, should override
		// --app-insights-key not specified
	})

	// Expected result:
	// - LogFetchSize: zero flag should not override, so env (222) wins
	// - Environment: empty flag should override env
	// - ApplicationInsightsKey: no flag, so env (env-key) wins
	if cfg.LogFetchSize != 222 {
		t.Errorf("Expected zero flag to not override env LogFetchSize, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "" {
		t.Errorf("Expected empty flag to override env Environment, got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "env-key" {
		t.Errorf("Expected env ApplicationInsightsKey when no flag set, got %q", cfg.ApplicationInsightsKey)
	}
}

func TestPrecedence_MultipleConfigFiles(t *testing.T) {
	// Test precedence when multiple config files exist (explicit vs discovered)
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Create auto-discovered config file in current directory
	autoConfigFile := "config.json"
	autoConfig := Config{
		LogFetchSize:           111,
		Environment:            "AutoEnv",
		ApplicationInsightsKey: "auto-key",
	}
	autoJsonData, _ := json.Marshal(autoConfig)
	os.WriteFile(autoConfigFile, autoJsonData, 0644)
	defer os.Remove(autoConfigFile)

	// Create explicit config file
	explicitConfigFile := filepath.Join(tempDir, "explicit-config.json")
	explicitConfig := Config{
		LogFetchSize:           222,
		Environment:            "ExplicitEnv",
		ApplicationInsightsKey: "explicit-key",
	}
	explicitJsonData, _ := json.Marshal(explicitConfig)
	os.WriteFile(explicitConfigFile, explicitJsonData, 0644)

	// Test explicit config file takes precedence over auto-discovered
	cfg := LoadConfigWithArgs([]string{"--config=" + explicitConfigFile})

	// Should get explicit config values, not auto-discovered
	if cfg.LogFetchSize != 222 {
		t.Errorf("Expected explicit config LogFetchSize 222, got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "ExplicitEnv" {
		t.Errorf("Expected explicit config Environment 'ExplicitEnv', got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "explicit-key" {
		t.Errorf("Expected explicit config ApplicationInsightsKey 'explicit-key', got %q", cfg.ApplicationInsightsKey)
	}

	// Test auto-discovery when no explicit config specified
	cfg2 := LoadConfigWithArgs([]string{})

	// Should get auto-discovered config values
	if cfg2.LogFetchSize != 111 {
		t.Errorf("Expected auto-discovered config LogFetchSize 111, got %d", cfg2.LogFetchSize)
	}
	if cfg2.Environment != "AutoEnv" {
		t.Errorf("Expected auto-discovered config Environment 'AutoEnv', got %q", cfg2.Environment)
	}
	if cfg2.ApplicationInsightsKey != "auto-key" {
		t.Errorf("Expected auto-discovered config ApplicationInsightsKey 'auto-key', got %q", cfg2.ApplicationInsightsKey)
	}
}

func TestPrecedence_ComplexScenario(t *testing.T) {
	// Test a complex real-world scenario with all sources active
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "complex-config.json")

	// Create config file with all values
	fileConfig := Config{
		LogFetchSize:           100,
		Environment:            "FileEnvironment",
		ApplicationInsightsKey: "file-key-123456789",
	}
	jsonData, _ := json.Marshal(fileConfig)
	os.WriteFile(configFile, jsonData, 0644)

	// Set environment variables for some values
	os.Setenv("LOG_FETCH_SIZE", "200")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key-987654321")
	// Note: BCINSIGHTS_ENVIRONMENT not set, so file should win
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	// Set command line flags for some values
	cfg := LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--fetch-size=300", // Flag should win
		// Note: --environment not set, so file should win since no env var
		"--app-insights-key=flag-key-111222333", // Flag should win
	})

	// Expected precedence results:
	// - LogFetchSize: flag (300) > env (200) > file (100) > default (50)
	// - Environment: file (FileEnvironment) > default (Development) [no env var or flag]
	// - ApplicationInsightsKey: flag (flag-key-111222333) > env (env-key-987654321) > file (file-key-123456789) > default ("")

	if cfg.LogFetchSize != 300 {
		t.Errorf("Expected flag to win for LogFetchSize (300), got %d", cfg.LogFetchSize)
	}
	if cfg.Environment != "FileEnvironment" {
		t.Errorf("Expected file to win for Environment (FileEnvironment), got %q", cfg.Environment)
	}
	if cfg.ApplicationInsightsKey != "flag-key-111222333" {
		t.Errorf("Expected flag to win for ApplicationInsightsKey (flag-key-111222333), got %q", cfg.ApplicationInsightsKey)
	}
}
