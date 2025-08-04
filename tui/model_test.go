package tui

import (
	"strings"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func TestInitialModel_ProperInitialization(t *testing.T) {
	cfg := config.NewConfig()
	cfg.LogFetchSize = 100

	model := InitialModel(cfg)

	// Test basic fields
	if model.WelcomeMsg != "Welcome to bc-insights-tui!" {
		t.Errorf("Expected welcome message to be 'Welcome to bc-insights-tui!', got %q", model.WelcomeMsg)
	}

	// Test help text includes config values
	expectedHelpSubstring := "Log fetch size: 100"
	if !strings.Contains(model.HelpText, expectedHelpSubstring) {
		t.Errorf("Expected help text to contain %q, got %q", expectedHelpSubstring, model.HelpText)
	}

	// Test config storage
	if model.Config.LogFetchSize != 100 {
		t.Errorf("Expected config LogFetchSize to be 100, got %d", model.Config.LogFetchSize)
	}

	// Test command palette initial state
	if model.CommandPalette {
		t.Error("Expected CommandPalette to be false initially")
	}

	if model.CommandInput != "" {
		t.Errorf("Expected CommandInput to be empty initially, got %q", model.CommandInput)
	}

	// Test default window dimensions
	if model.WindowWidth != 80 {
		t.Errorf("Expected default WindowWidth to be 80, got %d", model.WindowWidth)
	}

	if model.WindowHeight != 24 {
		t.Errorf("Expected default WindowHeight to be 24, got %d", model.WindowHeight)
	}
}

func TestInitialModel_HelpTextGeneration(t *testing.T) {
	testCases := []struct {
		name          string
		fetchSize     int
		shouldContain string
	}{
		{"default size", 50, "Log fetch size: 50"},
		{"custom size", 200, "Log fetch size: 200"},
		{"small size", 10, "Log fetch size: 10"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			cfg.LogFetchSize = tc.fetchSize
			model := InitialModel(cfg)

			if !strings.Contains(model.HelpText, tc.shouldContain) {
				t.Errorf("Expected help text to contain %q, got %q", tc.shouldContain, model.HelpText)
			}

			// Verify Ctrl+P instruction is included
			if !strings.Contains(model.HelpText, "Ctrl+P") {
				t.Errorf("Expected help text to contain Ctrl+P instruction, got %q", model.HelpText)
			}
		})
	}
}

func TestModel_Init(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	cmd := model.Init()

	// Init should return authentication check command
	if cmd == nil {
		t.Error("Expected Init() to return authentication check command, got nil")
	}
}

func TestModel_StatePreparation(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	// Test that model is prepared for command palette integration
	if model.CommandPalette {
		t.Error("CommandPalette should be false initially")
	}

	// Test that model can track window dimensions
	if model.WindowWidth <= 0 || model.WindowHeight <= 0 {
		t.Errorf("Window dimensions should be positive: %dx%d", model.WindowWidth, model.WindowHeight)
	}

	// Test command input is ready
	if model.CommandInput != "" {
		t.Errorf("CommandInput should be empty initially, got %q", model.CommandInput)
	}
}

func TestModel_ConfigIntegration(t *testing.T) {
	// Test that different config values are properly integrated
	testLogFetchSizes := []int{25, 100, 500}

	for _, fetchSize := range testLogFetchSizes {
		cfg := config.NewConfig()
		cfg.LogFetchSize = fetchSize
		model := InitialModel(cfg)

		// Verify config is stored
		if model.Config.LogFetchSize != fetchSize {
			t.Errorf("Expected stored config LogFetchSize to be %d, got %d",
				fetchSize, model.Config.LogFetchSize)
		}

		// Verify config is reflected in help text
		if !strings.Contains(model.HelpText, "Log fetch size: ") {
			t.Errorf("Help text should include log fetch size, got %q", model.HelpText)
		}
	}
}
