package config

import (
	"strings"
	"testing"
)

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// Test all validation rules comprehensively

func TestValidateAndUpdateSetting_FetchSize(t *testing.T) {
	cfg := NewConfig()
	cfg.LogFetchSize = 0 // Reset to zero for testing validation logic

	testCases := []struct {
		name          string
		value         string
		shouldError   bool
		expectedValue int
		errorContains string
	}{
		// Valid values
		{"valid positive integer", "100", false, 100, ""},
		{"valid small integer", "1", false, 1, ""},
		{"valid large integer", "99999", false, 99999, ""},
		{"valid with whitespace", " 50 ", false, 50, ""},

		// Invalid values
		{"negative integer", "-10", true, 0, "must be a positive integer"},
		{"zero", "0", true, 0, "must be a positive integer"},
		{"negative zero", "-0", true, 0, "must be a positive integer"},
		{"non-integer string", "abc", true, 0, "must be a positive integer"},
		{"mixed alphanumeric", "123abc", true, 0, "must be a positive integer"},
		{"float", "10.5", true, 0, "must be a positive integer"},
		{"empty string", "", true, 0, "must be a positive integer"},
		{"special characters", "!@#", true, 0, "must be a positive integer"},
		{"boolean", "true", true, 0, "must be a positive integer"},
		{"hex", "0xFF", true, 0, "must be a positive integer"},
		{"scientific notation", "1e2", true, 0, "must be a positive integer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset config for each test
			cfg = NewConfig()
			cfg.LogFetchSize = 0

			err := cfg.ValidateAndUpdateSetting("fetchSize", tc.value)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for value %q, got none", tc.value)
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
				// Config should not be updated on error
				if cfg.LogFetchSize != 0 {
					t.Errorf("Expected config to not be updated on error, got LogFetchSize=%d", cfg.LogFetchSize)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for value %q, got: %v", tc.value, err)
				}
				// Config should be updated on success
				if cfg.LogFetchSize != tc.expectedValue {
					t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expectedValue, cfg.LogFetchSize)
				}
			}
		})
	}
}

func TestValidateAndUpdateSetting_Environment(t *testing.T) {
	cfg := NewConfig()

	testCases := []struct {
		name          string
		value         string
		shouldError   bool
		expectedValue string
		errorContains string
	}{
		// Valid values
		{"simple environment", "Production", false, "Production", ""},
		{"development environment", "Development", false, "Development", ""},
		{"testing environment", "Testing", false, "Testing", ""},
		{"staging environment", "Staging", false, "Staging", ""},
		{"custom environment", "MyCustomEnv", false, "MyCustomEnv", ""},
		{"environment with numbers", "Prod2024", false, "Prod2024", ""},
		{"environment with underscores", "Test_Environment", false, "Test_Environment", ""},
		{"environment with hyphens", "Test-Environment", false, "Test-Environment", ""},
		{"environment with spaces", "Test Environment", false, "Test Environment", ""},
		{"environment with dots", "env.production", false, "env.production", ""},
		{"single character", "P", false, "P", ""},
		{"long environment name", "VeryLongEnvironmentNameForTesting", false, "VeryLongEnvironmentNameForTesting", ""},
		{"mixed case", "pRoDuCtIoN", false, "pRoDuCtIoN", ""},
		{"with leading/trailing spaces", " Production ", false, " Production ", ""}, // Trimming should be handled by caller

		// Invalid values
		{"empty string", "", true, "", "cannot be empty"},
		{"only whitespace", "   ", true, "", "cannot be empty"}, // Should be trimmed by caller, but let's test
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset config for each test
			cfg = NewConfig()
			cfg.LogFetchSize = 0

			err := cfg.ValidateAndUpdateSetting("environment", tc.value)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for value %q, got none", tc.value)
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
				// Config should not be updated on error (but default is empty string)
			} else {
				if err != nil {
					t.Errorf("Expected no error for value %q, got: %v", tc.value, err)
				}
				// Config should be updated on success
				if cfg.Environment != tc.expectedValue {
					t.Errorf("Expected Environment to be %q, got %q", tc.expectedValue, cfg.Environment)
				}
			}
		})
	}
}

func TestValidateAndUpdateSetting_ApplicationInsightsKey(t *testing.T) {
	cfg := NewConfig()

	testCases := []struct {
		name          string
		value         string
		shouldError   bool
		expectedValue string
	}{
		// Valid values - applicationInsightsKey allows any value including empty
		{"instrumentation key format", "InstrumentationKey=12345678-1234-1234-1234-123456789012", false, "InstrumentationKey=12345678-1234-1234-1234-123456789012"},
		{"connection string format", "InstrumentationKey=abc;IngestionEndpoint=https://example.com", false, "InstrumentationKey=abc;IngestionEndpoint=https://example.com"},
		{"simple key", "simple-key-123", false, "simple-key-123"},
		{"guid only", "12345678-1234-1234-1234-123456789012", false, "12345678-1234-1234-1234-123456789012"},
		{"long key", "very-long-application-insights-key-with-many-characters-and-dashes", false, "very-long-application-insights-key-with-many-characters-and-dashes"},
		{"key with special characters", "key!@#$%^&*()_+-=[]{}|;:,.<>?", false, "key!@#$%^&*()_+-=[]{}|;:,.<>?"},
		{"key with spaces", "key with spaces", false, "key with spaces"},
		{"empty string", "", false, ""},          // Allowed for clearing the key
		{"only whitespace", "   ", false, "   "}, // Even whitespace is allowed
		{"unicode characters", "key-with-üñíçødé", false, "key-with-üñíçødé"},
		{"newlines", "key\nwith\nnewlines", false, "key\nwith\nnewlines"},
		{"tabs", "key\twith\ttabs", false, "key\twith\ttabs"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset config for each test
			cfg = NewConfig()
			cfg.LogFetchSize = 0

			err := cfg.ValidateAndUpdateSetting("applicationInsightsKey", tc.value)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for value %q, got none", tc.value)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for value %q, got: %v", tc.value, err)
				}
				// Config should be updated
				if cfg.ApplicationInsightsKey != tc.expectedValue {
					t.Errorf("Expected ApplicationInsightsKey to be %q, got %q", tc.expectedValue, cfg.ApplicationInsightsKey)
				}
			}
		})
	}
}

func TestValidateAndUpdateSetting_UnknownSetting(t *testing.T) {
	cfg := NewConfig()

	unknownSettings := []string{
		"unknownSetting",
		"invalidSetting",
		"nonExistentField",
		"fetchsize",              // Wrong case
		"FetchSize",              // Wrong case
		"FETCHSIZE",              // Wrong case
		"Environment",            // Wrong case
		"applicationinsightskey", // Wrong case
		"app-insights-key",       // Wrong format
		"logFetchSize",           // Wrong field name
		"",
		"123",
		"setting-with-dashes",
		"setting_with_underscores",
		"setting with spaces",
	}

	for _, setting := range unknownSettings {
		t.Run("unknown_setting_"+setting, func(t *testing.T) {
			// Capture original values
			originalFetchSize := cfg.LogFetchSize
			originalEnvironment := cfg.Environment
			originalAppInsightsKey := cfg.ApplicationInsightsKey
			originalTenantID := cfg.OAuth2.TenantID
			originalClientID := cfg.OAuth2.ClientID
			originalScopes := append([]string(nil), cfg.OAuth2.Scopes...) // Create a copy

			err := cfg.ValidateAndUpdateSetting(setting, "some-value")

			if err == nil {
				t.Errorf("Expected error for unknown setting %q, got none", setting)
			}

			expectedError := "unknown setting: " + setting
			if !strings.Contains(err.Error(), expectedError) {
				t.Errorf("Expected error containing %q, got %q", expectedError, err.Error())
			}

			// Config should not be modified for unknown settings
			if cfg.LogFetchSize != originalFetchSize ||
				cfg.Environment != originalEnvironment ||
				cfg.ApplicationInsightsKey != originalAppInsightsKey ||
				cfg.OAuth2.TenantID != originalTenantID ||
				cfg.OAuth2.ClientID != originalClientID ||
				!equalStringSlices(cfg.OAuth2.Scopes, originalScopes) {
				t.Errorf("Expected config to remain unchanged for unknown setting %q", setting)
			}
		})
	}
}

func TestGetSettingValue_AllSettings(t *testing.T) {
	cfg := NewConfig()
	cfg.LogFetchSize = 123
	cfg.Environment = "TestValue"
	cfg.ApplicationInsightsKey = "test-key-123456789"

	testCases := []struct {
		setting       string
		expectedValue string
		shouldError   bool
	}{
		{"fetchSize", "123", false},
		{"environment", "TestValue", false},
		{"applicationInsightsKey", "test...6789", false}, // Should be masked (first 4 + last 4)
		{"unknownSetting", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.setting, func(t *testing.T) {
			value, err := cfg.GetSettingValue(tc.setting)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for setting %q, got none", tc.setting)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for setting %q, got: %v", tc.setting, err)
				}
				if value != tc.expectedValue {
					t.Errorf("Expected value %q for setting %q, got %q", tc.expectedValue, tc.setting, value)
				}
			}
		})
	}
}

func TestGetSettingValue_ApplicationInsightsKeyMasking(t *testing.T) {
	testCases := []struct {
		name         string
		key          string
		expectedMask string
	}{
		{"empty key", "", "(not set)"},
		{"short key", "short", "***"},
		{"8 character key", "12345678", "***"},
		{"9 character key", "123456789", "1234...6789"},
		{"long key", "InstrumentationKey=12345678-1234-1234-1234-123456789012", "Inst...9012"},
		{"very long key", "very-long-application-insights-connection-string-with-many-parts", "very...arts"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.ApplicationInsightsKey = tc.key

			value, err := cfg.GetSettingValue("applicationInsightsKey")
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if value != tc.expectedMask {
				t.Errorf("Expected masked value %q, got %q", tc.expectedMask, value)
			}
		})
	}
}

func TestListAllSettings(t *testing.T) {
	cfg := NewConfig()
	cfg.LogFetchSize = 456
	cfg.Environment = "ListTestEnv"
	cfg.ApplicationInsightsKey = "list-test-key-123456789"

	settings := cfg.ListAllSettings()

	// Check that all expected settings are present
	expectedSettings := []string{"fetchSize", "environment", "applicationInsightsKey", "oauth2.tenantId", "oauth2.clientId", "oauth2.scopes"}
	for _, expectedSetting := range expectedSettings {
		if _, exists := settings[expectedSetting]; !exists {
			t.Errorf("Expected setting %q to be present in ListAllSettings", expectedSetting)
		}
	}

	// Check values
	if settings["fetchSize"] != "456" {
		t.Errorf("Expected fetchSize to be '456', got %q", settings["fetchSize"])
	}
	if settings["environment"] != "ListTestEnv" {
		t.Errorf("Expected environment to be 'ListTestEnv', got %q", settings["environment"])
	}
	if settings["applicationInsightsKey"] != "list...6789" {
		t.Errorf("Expected applicationInsightsKey to be masked, got %q", settings["applicationInsightsKey"])
	}
	// Check OAuth2 default values
	if settings["oauth2.tenantId"] != "e48da249-7c64-41ec-8c89-cea18b6608fa" {
		t.Errorf("Expected oauth2.tenantId to be the default value, got %q", settings["oauth2.tenantId"])
	}
	if settings["oauth2.clientId"] != "3b065ad6-067e-41f2-8cf7-19ddb0548a99" {
		t.Errorf("Expected oauth2.clientId to be the default value, got %q", settings["oauth2.clientId"])
	}
	if settings["oauth2.scopes"] != "https://api.applicationinsights.io/Data.Read" {
		t.Errorf("Expected oauth2.scopes to be the default value, got %q", settings["oauth2.scopes"])
	}

	// Check that no unexpected settings are present
	if len(settings) != 6 {
		t.Errorf("Expected exactly 6 settings, got %d: %v", len(settings), settings)
	}
}

func TestListAllSettings_EmptyAndShortKeys(t *testing.T) {
	testCases := []struct {
		name                   string
		applicationInsightsKey string
		expectedMask           string
	}{
		{"empty key", "", "(not set)"},
		{"short key", "abc", "***"},
		{"exactly 8 chars", "12345678", "***"},
		{"9 chars", "123456789", "1234...6789"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.LogFetchSize = 100
			cfg.Environment = "Test"
			cfg.ApplicationInsightsKey = tc.applicationInsightsKey

			settings := cfg.ListAllSettings()

			if settings["applicationInsightsKey"] != tc.expectedMask {
				t.Errorf("Expected applicationInsightsKey to be %q, got %q", tc.expectedMask, settings["applicationInsightsKey"])
			}
		})
	}
}

func TestValidation_EdgeCases(t *testing.T) {
	cfg := NewConfig()

	// Test edge cases that might cause panics or unexpected behavior
	edgeCases := []struct {
		setting string
		value   string
		desc    string
	}{
		{"fetchSize", "9223372036854775807", "max int64"},                         // Should work
		{"fetchSize", "9223372036854775808", "overflow int64"},                    // Should fail
		{"environment", strings.Repeat("a", 10000), "very long environment name"}, // Should work
		{"applicationInsightsKey", strings.Repeat("x", 10000), "very long key"},   // Should work
	}

	for _, ec := range edgeCases {
		t.Run(ec.desc, func(t *testing.T) {
			// Should not panic
			err := cfg.ValidateAndUpdateSetting(ec.setting, ec.value)

			// Just verify it doesn't crash - specific behavior depends on the case
			t.Logf("Edge case %q with setting %q and value length %d: error = %v",
				ec.desc, ec.setting, len(ec.value), err)
		})
	}
}

func TestValidation_ConcurrentAccess(t *testing.T) {
	// Test that validation is safe for concurrent access (no race conditions)
	cfg := NewConfig()
	cfg.LogFetchSize = 50
	cfg.Environment = "Concurrent"
	cfg.ApplicationInsightsKey = "concurrent-key"

	// Run multiple goroutines to test concurrent access
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Each goroutine performs various operations
			cfg.ValidateAndUpdateSetting("fetchSize", "100")
			cfg.GetSettingValue("environment")
			cfg.ListAllSettings()
			cfg.ValidateAndUpdateSetting("environment", "ConcurrentTest")
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without hanging or panicking, the test passes
	t.Log("Concurrent access test completed successfully")
}
