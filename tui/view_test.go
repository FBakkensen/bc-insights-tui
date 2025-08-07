package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func TestView_BasicOutput(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Test welcome message is included
	if !strings.Contains(output, "Welcome to bc-insights-tui!") {
		t.Errorf("Expected output to contain welcome message")
	}

	// Test Business Central context is included
	if !strings.Contains(output, "Business Central") {
		t.Errorf("Expected output to contain Business Central context")
	}

	// Test essential keyboard shortcuts are included
	if !strings.Contains(output, "Ctrl+P") {
		t.Errorf("Expected output to contain Ctrl+P shortcut")
	}

	// Test configuration is displayed
	if !strings.Contains(output, "Log fetch size: 50") {
		t.Errorf("Expected output to contain log fetch size configuration")
	}

	// Test basic structure - should have header, content area, and footer
	lines := strings.Split(output, "\n")
	if len(lines) < 10 {
		t.Errorf("Expected at least 10 lines in full-screen output, got %d", len(lines))
	}
}

func TestView_TerminalSizeDisplay(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)
	model.WindowWidth = 100
	model.WindowHeight = 30

	output := model.View()

	expectedSize := "Terminal: 100x30"
	if !strings.Contains(output, expectedSize) {
		t.Errorf("Expected output to contain %q, got:\n%s", expectedSize, output)
	}
}

func TestView_ConfigValues(t *testing.T) {
	testCases := []struct {
		name      string
		fetchSize int
	}{
		{"default size", 50},
		{"custom size", 100},
		{"large size", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			cfg.LogFetchSize = tc.fetchSize
			model := InitialModel(cfg)

			output := model.View()

			expectedConfig := fmt.Sprintf("Log fetch size: %d", tc.fetchSize)
			if !strings.Contains(output, expectedConfig) {
				t.Errorf("Expected output to contain %q, got:\n%s", expectedConfig, output)
			}
		})
	}
}

func TestView_CommandPaletteClosed(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)
	model.CommandPalette = false

	output := model.View()

	// Command palette overlay should not be visible
	if strings.Contains(output, "Press Esc to close, Enter to execute") {
		t.Errorf("Expected no command palette overlay in output when closed, got:\n%s", output)
	}

	if strings.Contains(output, "╔════") {
		t.Errorf("Expected no command palette borders when palette is closed, got:\n%s", output)
	}
}

func TestView_CommandPaletteOpen(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = ""

	output := model.View()

	// Command palette should be visible
	if !strings.Contains(output, "Command Palette") {
		t.Errorf("Expected command palette header in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Press Esc to close") {
		t.Errorf("Expected escape instruction in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Enter to execute") {
		t.Errorf("Expected enter instruction in output, got:\n%s", output)
	}

	// Should show input prompt
	if !strings.Contains(output, ">") {
		t.Errorf("Expected input prompt '>' in output, got:\n%s", output)
	}
}

func TestView_CommandPaletteWithInput(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = "filter: test"

	output := model.View()

	// Should show the command input
	if !strings.Contains(output, "filter: test") {
		t.Errorf("Expected command input 'filter: test' in output, got:\n%s", output)
	}

	// Should show input with prompt
	if !strings.Contains(output, "> filter: test") {
		t.Errorf("Expected command input with prompt '> filter: test' in output, got:\n%s", output)
	}
}

func TestView_WindowSizeVariations(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"small terminal", 40, 10},
		{"default terminal", 80, 24},
		{"large terminal", 120, 40},
		{"very wide terminal", 200, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			model := InitialModel(cfg)
			model.WindowWidth = tc.width
			model.WindowHeight = tc.height

			output := model.View()

			// Verify terminal size is displayed correctly in footer
			expectedTerminalSize := fmt.Sprintf("Terminal: %dx%d", tc.width, tc.height)
			if !strings.Contains(output, expectedTerminalSize) {
				t.Errorf("Expected terminal size %s in output for %s, got:\n%s", expectedTerminalSize, tc.name, output)
			}

			// Verify basic layout elements are present
			if !strings.Contains(output, "bc-insights-tui") {
				t.Errorf("Expected application name in output for %s", tc.name)
			}

			if !strings.Contains(output, "Welcome to bc-insights-tui!") {
				t.Errorf("Expected welcome message in output for %s", tc.name)
			}

			// Test command palette works with different sizes
			model.CommandPalette = true
			model.CommandInput = testCommandInput
			output = model.View()

			// Command palette should appear in overlay regardless of size
			if !strings.Contains(output, "Command Palette") {
				t.Errorf("Expected command palette in output for %s when palette is open", tc.name)
			}

			if !strings.Contains(output, "> "+testCommandInput) {
				t.Errorf("Expected command input in palette for %s", tc.name)
			}
		})
	}
}

func TestView_ConsistentStructure(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	// Test without command palette
	output1 := model.View()

	// Test with command palette
	model.CommandPalette = true
	model.CommandInput = testCommandInput
	output2 := model.View()

	// Both outputs should contain the basic elements
	basicElements := []string{
		"Welcome to bc-insights-tui!",
		"Ctrl+P",
		"Terminal:",
		"Business Central",
	}

	for _, element := range basicElements {
		if !strings.Contains(output1, element) {
			t.Errorf("Expected basic output to contain %q, got:\n%s", element, output1)
		}

		if !strings.Contains(output2, element) {
			t.Errorf("Expected command palette output to contain %q, got:\n%s", element, output2)
		}
	}

	// Only the command palette version should have palette overlay elements
	paletteElements := []string{
		"╔════",
		"Press Esc to close, Enter to execute",
		"> " + testCommandInput,
	}

	for _, element := range paletteElements {
		if strings.Contains(output1, element) {
			t.Errorf("Expected basic output to NOT contain %q, got:\n%s", element, output1)
		}

		if !strings.Contains(output2, element) {
			t.Errorf("Expected command palette output to contain %q, got:\n%s", element, output2)
		}
	}
}

func TestView_StringFormatting(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Verify proper newline usage
	if !strings.HasSuffix(output, "\n") {
		t.Error("Expected output to end with newline")
	}

	// Verify no empty lines at start
	if strings.HasPrefix(output, "\n") {
		t.Error("Expected output to not start with newline")
	}

	// Test that output doesn't have excessive blank lines
	lines := strings.Split(output, "\n")
	consecutiveEmpty := 0
	maxConsecutiveEmpty := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			consecutiveEmpty++
			if consecutiveEmpty > maxConsecutiveEmpty {
				maxConsecutiveEmpty = consecutiveEmpty
			}
		} else {
			consecutiveEmpty = 0
		}
	}

	// Allow some empty lines for formatting, but not excessive
	if maxConsecutiveEmpty > 3 {
		t.Errorf("Expected no more than 3 consecutive empty lines, found %d in:\n%s",
			maxConsecutiveEmpty, output)
	}
}

// New tests for full-screen welcome view functionality

func TestView_FullScreenLayout(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)
	model.WindowWidth = 120
	model.WindowHeight = 40

	output := model.View()

	// Test header bar with application title and version
	if !strings.Contains(output, "bc-insights-tui v1.0.0") {
		t.Errorf("Expected header to contain application name and version")
	}

	if !strings.Contains(output, "Welcome - Business Central Developer Tools") {
		t.Errorf("Expected header to contain Business Central context")
	}

	// Test main content area structure
	if !strings.Contains(output, "╭") || !strings.Contains(output, "╰") {
		t.Errorf("Expected bordered layout with rounded corners")
	}

	// Test footer bar with keyboard shortcuts and system info
	if !strings.Contains(output, "[Ctrl+P] Open Command Palette") {
		t.Errorf("Expected footer to contain command palette shortcut")
	}

	if !strings.Contains(output, "[Ctrl+C] Quit") {
		t.Errorf("Expected footer to contain quit shortcut")
	}

	if !strings.Contains(output, "Terminal: 120x40") {
		t.Errorf("Expected footer to contain terminal size")
	}
}

func TestView_ApplicationVersionDisplay(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	if !strings.Contains(output, AppVersion) {
		t.Errorf("Expected output to contain application version %s", AppVersion)
	}

	if !strings.Contains(output, "bc-insights-tui v1.0.0") {
		t.Errorf("Expected output to contain formatted application name with version")
	}
}

func TestView_BusinessCentralContext(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Test Business Central context is prominently displayed
	if !strings.Contains(output, "Business Central Developer Tools") {
		t.Errorf("Expected header to emphasize Business Central developer tools")
	}

	if !strings.Contains(output, "Microsoft Dynamics 365 Business Central") {
		t.Errorf("Expected main content to mention target audience")
	}

	if !strings.Contains(output, "command palette") && !strings.Contains(output, "workflow") {
		t.Errorf("Expected content to describe the command palette workflow")
	}
}

func TestView_AuthenticationStatus(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	// Model starts with AuthStateUnknown, should show main view
	output := model.View()

	// Test that auth configuration is shown
	if !strings.Contains(output, "Azure Tenant ID") {
		t.Error("Expected Azure Tenant ID to be displayed")
	}

	if !strings.Contains(output, "Azure Client ID") {
		t.Error("Expected Azure Client ID to be displayed")
	}
}

func TestView_ApplicationStateDisplay(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Test current state is clearly displayed
	if !strings.Contains(output, "Status:") {
		t.Error("Expected status display")
	}

	// Should show authentication status
	if !strings.Contains(output, "Authentication") || !strings.Contains(output, "required") {
		t.Error("Expected to show authentication status")
	}
}

func TestView_EnhancedContent(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Test enhanced welcome messaging
	if !strings.Contains(output, "Terminal User Interface for Azure Application Insights") {
		t.Errorf("Expected detailed application description")
	}

	// Test AI feature preview
	if !strings.Contains(output, "AI-powered KQL query generation") {
		t.Errorf("Expected AI feature preview")
	}

	if !strings.Contains(output, "Phase 5") {
		t.Errorf("Expected phase information for AI features")
	}

	if !strings.Contains(output, "natural language to generate complex queries") {
		t.Errorf("Expected AI feature explanation")
	}

	// Test comprehensive keyboard shortcuts
	shortcuts := []string{"Ctrl+P", "Ctrl+C", "q", "Esc"}
	for _, shortcut := range shortcuts {
		if !strings.Contains(output, shortcut) {
			t.Errorf("Expected keyboard shortcut %s to be documented", shortcut)
		}
	}
}

func TestView_ResponsiveDesign(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"very small", 40, 10},
		{"small", 60, 20},
		{"medium", 80, 24},
		{"large", 120, 40},
		{"very large", 200, 60},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			model := InitialModel(cfg)
			model.WindowWidth = tc.width
			model.WindowHeight = tc.height

			output := model.View()

			// Layout should adapt to terminal size
			expectedTerminalSize := fmt.Sprintf("%dx%d", tc.width, tc.height)
			if !strings.Contains(output, expectedTerminalSize) {
				t.Errorf("Expected terminal size %s in output for %s, got:\n%s", expectedTerminalSize, tc.name, output)
			}

			// Essential elements should be present regardless of size
			essentialElements := []string{
				"bc-insights-tui",
				"Welcome to bc-insights-tui!",
				"Ctrl+P",
				"Business Central",
			}

			for _, element := range essentialElements {
				if !strings.Contains(output, element) {
					t.Errorf("Expected essential element %q to be present in %s terminal", element, tc.name)
				}
			}
		})
	}
}

func TestView_ColorSchemeCompatibility(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Test that output doesn't contain problematic characters or sequences
	// that might not work in different terminal environments
	if strings.Contains(output, "\x1b[") {
		// This would indicate raw ANSI escape sequences, which lipgloss should handle
		t.Logf("Note: ANSI escape sequences detected - this is expected with lipgloss styling")
	}

	// Basic structure should work in any terminal
	if !strings.Contains(output, "Welcome to bc-insights-tui!") {
		t.Errorf("Expected basic text content to be present")
	}
}

func TestView_ProfessionalStyling(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	output := model.View()

	// Test professional visual elements are present
	stylingElements := []string{
		"╭", "╰", "│", // Box drawing characters for borders
		"🔧", "📋", "🤖", "⚙️", // Icons for visual hierarchy
		"•", // Bullet points for lists
	}

	for _, element := range stylingElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected styling element %q for professional appearance", element)
		}
	}

	// Test content organization with proper spacing
	lines := strings.Split(output, "\n")
	if len(lines) < 20 {
		t.Errorf("Expected full-screen layout to have substantial content, got %d lines", len(lines))
	}
}

// Additional view tests as requested in the issue

func TestView_AllConfigurationDisplay(t *testing.T) {
	cfg := config.NewConfig()
	cfg.LogFetchSize = 75
	cfg.Environment = "ConfigTest"
	cfg.ApplicationInsightsKey = "display-test-key"
	model := InitialModel(cfg)

	output := model.View()

	// Test all configuration settings are displayed
	expectedDisplays := []string{
		"Log fetch size: 75",
		"Environment: ConfigTest",
		"Application Insights Key: disp...-key", // Should be masked - actual output shows "disp...-key"
	}

	for _, expected := range expectedDisplays {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected configuration display to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestView_ConfigurationFormatting(t *testing.T) {
	testCases := []struct {
		name                   string
		logFetchSize           int
		environment            string
		applicationInsightsKey string
		expected               []string
	}{
		{
			name:                   "standard configuration",
			logFetchSize:           100,
			environment:            "Production",
			applicationInsightsKey: "InstrumentationKey=abc123def456",
			expected: []string{
				"Log fetch size: 100",
				"Environment: Production",
				"Application Insights Key: Inst...f456", // Masked - actual output
			},
		},
		{
			name:                   "short key configuration",
			logFetchSize:           25,
			environment:            "Dev",
			applicationInsightsKey: "short",
			expected: []string{
				"Log fetch size: 25",
				"Environment: Dev",
				"Application Insights Key: ***", // Short key masked
			},
		},
		{
			name:                   "empty key configuration",
			logFetchSize:           200,
			environment:            "Testing",
			applicationInsightsKey: "",
			expected: []string{
				"Log fetch size: 200",
				"Environment: Testing",
				"Application Insights Key: (not set)", // Empty key
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			cfg.LogFetchSize = tc.logFetchSize
			cfg.Environment = tc.environment
			cfg.ApplicationInsightsKey = tc.applicationInsightsKey
			model := InitialModel(cfg)
			output := model.View()

			for _, expected := range tc.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestView_ConfigurationUpdates(t *testing.T) {
	// Test that view updates when configuration changes
	cfg := config.NewConfig()
	cfg.Environment = "Original"
	model := InitialModel(cfg)

	// Get initial view
	initialOutput := model.View()
	if !strings.Contains(initialOutput, "Log fetch size: 50") {
		t.Error("Expected initial view to show original fetch size")
	}
	if !strings.Contains(initialOutput, "Environment: Original") {
		t.Error("Expected initial view to show original environment")
	}

	// Update configuration
	model.Config.LogFetchSize = 150
	model.Config.Environment = "Updated"
	model.HelpText = fmt.Sprintf("Press Ctrl+Q to quit, Ctrl+P for command palette. Log fetch size: %d", model.Config.LogFetchSize)

	// Get updated view
	updatedOutput := model.View()
	if !strings.Contains(updatedOutput, "Log fetch size: 150") {
		t.Error("Expected updated view to show new fetch size")
	}
	if !strings.Contains(updatedOutput, "Environment: Updated") {
		t.Error("Expected updated view to show new environment")
	}

	// Ensure old values are not present
	if strings.Contains(updatedOutput, "Log fetch size: 50") {
		t.Error("Expected updated view to not show old fetch size")
	}
	if strings.Contains(updatedOutput, "Environment: Original") {
		t.Error("Expected updated view to not show old environment")
	}
}

func TestView_SetCommandFeedback(t *testing.T) {
	cfg := config.NewConfig()
	model := InitialModel(cfg)

	testCases := []struct {
		name             string
		feedbackMessage  string
		feedbackIsError  bool
		shouldContain    string
		shouldNotContain string
	}{
		{
			name:            "success feedback",
			feedbackMessage: "✓ fetchSize set to: 100",
			feedbackIsError: false,
			shouldContain:   "✓ fetchSize set to: 100",
		},
		{
			name:            "error feedback",
			feedbackMessage: "fetchSize must be a positive integer, got: -10",
			feedbackIsError: true,
			shouldContain:   "fetchSize must be a positive integer",
		},
		{
			name:            "list settings feedback",
			feedbackMessage: "Current settings: fetchSize=50, environment=Development, applicationInsightsKey=(not set)",
			feedbackIsError: false,
			shouldContain:   "Current settings:",
		},
		{
			name:             "no feedback",
			feedbackMessage:  "",
			feedbackIsError:  false,
			shouldNotContain: "✓",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set feedback on model
			model.FeedbackMessage = tc.feedbackMessage
			model.FeedbackIsError = tc.feedbackIsError

			output := model.View()

			if tc.shouldContain != "" {
				if !strings.Contains(output, tc.shouldContain) {
					t.Errorf("Expected output to contain %q, got:\n%s", tc.shouldContain, output)
				}
			}

			if tc.shouldNotContain != "" {
				if strings.Contains(output, tc.shouldNotContain) {
					t.Errorf("Expected output to NOT contain %q, got:\n%s", tc.shouldNotContain, output)
				}
			}

			// Test that error feedback is visually distinct (if there's error styling)
			if tc.feedbackIsError && tc.feedbackMessage != "" {
				// Error feedback should be present in output
				if !strings.Contains(output, tc.feedbackMessage) {
					t.Errorf("Expected error feedback %q to be visible in output", tc.feedbackMessage)
				}
			}
		})
	}
}
