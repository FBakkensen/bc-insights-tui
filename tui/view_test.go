package tui

import (
	"strings"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func TestView_BasicOutput(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)

	output := model.View()

	// Test welcome message is included
	if !strings.Contains(output, model.WelcomeMsg) {
		t.Errorf("Expected output to contain welcome message %q", model.WelcomeMsg)
	}

	// Test help text is included
	if !strings.Contains(output, model.HelpText) {
		t.Errorf("Expected output to contain help text %q", model.HelpText)
	}

	// Test basic structure
	lines := strings.Split(output, "\n")
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines in output, got %d", len(lines))
	}
}

func TestView_TerminalSizeDisplay(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.WindowWidth = 100
	model.WindowHeight = 30

	output := model.View()

	expectedSize := "Terminal size: 100x30"
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
			cfg := config.Config{LogFetchSize: tc.fetchSize}
			model := InitialModel(cfg)

			output := model.View()

			if !strings.Contains(output, "Log fetch size:") {
				t.Errorf("Expected output to contain log fetch size information, got:\n%s", output)
			}
		})
	}
}

func TestView_CommandPaletteClosed(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = false

	output := model.View()

	// Command palette should not be visible
	if strings.Contains(output, "Command Palette") {
		t.Errorf("Expected no command palette in output when closed, got:\n%s", output)
	}

	if strings.Contains(output, "press Esc to close") {
		t.Errorf("Expected no escape instruction when palette is closed, got:\n%s", output)
	}
}

func TestView_CommandPaletteOpen(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = ""

	output := model.View()

	// Command palette should be visible
	if !strings.Contains(output, "Command Palette") {
		t.Errorf("Expected command palette header in output, got:\n%s", output)
	}

	if !strings.Contains(output, "press Esc to close") {
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
	cfg := config.Config{LogFetchSize: 50}
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
			cfg := config.Config{LogFetchSize: 50}
			model := InitialModel(cfg)
			model.WindowWidth = tc.width
			model.WindowHeight = tc.height

			output := model.View()

			// Verify terminal size is displayed correctly
			if !strings.Contains(output, "Terminal size:") {
				t.Errorf("Expected terminal size in output for %s, got:\n%s", tc.name, output)
			}

			// Test command palette border respects width
			model.CommandPalette = true
			output = model.View()

			// The border should not exceed window width or 50 characters
			expectedMaxBorder := min(tc.width, 50)
			borderLine := strings.Repeat("─", expectedMaxBorder)

			if tc.width >= 50 && !strings.Contains(output, borderLine) {
				t.Errorf("Expected border of length %d for width %d, got output:\n%s",
					expectedMaxBorder, tc.width, output)
			}
		})
	}
}

func TestView_ConsistentStructure(t *testing.T) {
	cfg := config.Config{LogFetchSize: 75}
	model := InitialModel(cfg)

	// Test without command palette
	output1 := model.View()

	// Test with command palette
	model.CommandPalette = true
	model.CommandInput = "test"
	output2 := model.View()

	// Both outputs should contain the basic elements
	basicElements := []string{
		model.WelcomeMsg,
		"Press q to quit",
		"Terminal size:",
	}

	for _, element := range basicElements {
		if !strings.Contains(output1, element) {
			t.Errorf("Expected basic output to contain %q, got:\n%s", element, output1)
		}

		if !strings.Contains(output2, element) {
			t.Errorf("Expected command palette output to contain %q, got:\n%s", element, output2)
		}
	}

	// Only the command palette version should have palette elements
	paletteElements := []string{
		"Command Palette",
		"press Esc to close",
		"> test",
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
	cfg := config.Config{LogFetchSize: 50}
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
