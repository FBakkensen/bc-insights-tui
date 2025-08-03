package tui

import (
	"strings"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdate_QuitCommands(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)

	quitCommands := []string{"q", "ctrl+c"}

	for _, cmd := range quitCommands {
		t.Run(cmd, func(t *testing.T) {
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(cmd)}
			if cmd == "ctrl+c" {
				keyMsg = tea.KeyMsg{Type: tea.KeyCtrlC}
			}

			newModel, teaCmd := model.Update(keyMsg)

			if teaCmd == nil {
				t.Errorf("Expected %q to trigger quit command, got nil", cmd)
			}

			// Model should be returned as-is
			if newModel != model {
				t.Errorf("Expected model to remain unchanged on quit")
			}
		})
	}
}

func TestUpdate_CommandPaletteTrigger(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)

	// Test Ctrl+P opens command palette
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlP}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on Ctrl+P, got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if !updatedModel.CommandPalette {
		t.Error("Expected CommandPalette to be true after Ctrl+P")
	}

	if updatedModel.CommandInput != "" {
		t.Errorf("Expected CommandInput to be empty after opening palette, got %q", updatedModel.CommandInput)
	}
}

func TestUpdate_CommandPaletteInput(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true // Open command palette

	// Test typing in command palette
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on character input, got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if updatedModel.CommandInput != "a" {
		t.Errorf("Expected CommandInput to be 'a', got %q", updatedModel.CommandInput)
	}

	// Test multiple characters
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
	newModel, _ = updatedModel.Update(keyMsg)
	updatedModel = newModel.(Model)

	if updatedModel.CommandInput != "ai" {
		t.Errorf("Expected CommandInput to be 'ai', got %q", updatedModel.CommandInput)
	}
}

func TestUpdate_CommandPaletteBackspace(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = testCommandInput

	// Test backspace
	keyMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on backspace, got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if updatedModel.CommandInput != "tes" {
		t.Errorf("Expected CommandInput to be 'tes' after backspace, got %q", updatedModel.CommandInput)
	}

	// Test backspace on empty input
	emptyModel := model
	emptyModel.CommandInput = ""
	newModel, _ = emptyModel.Update(keyMsg)
	updatedModel = newModel.(Model)

	if updatedModel.CommandInput != "" {
		t.Errorf("Expected CommandInput to remain empty after backspace on empty input, got %q", updatedModel.CommandInput)
	}
}

func TestUpdate_CommandPaletteEscape(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = "some command"

	// Test escape closes command palette
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on escape, got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if updatedModel.CommandPalette {
		t.Error("Expected CommandPalette to be false after escape")
	}

	if updatedModel.CommandInput != "" {
		t.Errorf("Expected CommandInput to be empty after escape, got %q", updatedModel.CommandInput)
	}
}

func TestUpdate_CommandPaletteEnter(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = "filter: test"

	// Test enter processes command (placeholder functionality)
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on enter (placeholder), got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if updatedModel.CommandPalette {
		t.Error("Expected CommandPalette to be false after enter")
	}

	if updatedModel.CommandInput != "" {
		t.Errorf("Expected CommandInput to be empty after enter, got %q", updatedModel.CommandInput)
	}
}

func TestUpdate_EscapeInMainMode(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = false // Ensure we're in main mode

	// Test escape in main mode (should do nothing for now)
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on escape in main mode, got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if updatedModel.CommandPalette {
		t.Error("Expected CommandPalette to remain false in main mode")
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)

	// Test window resize
	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, cmd := model.Update(resizeMsg)

	if cmd != nil {
		t.Errorf("Expected no command on window resize, got %v", cmd)
	}

	updatedModel := newModel.(Model)
	if updatedModel.WindowWidth != 120 {
		t.Errorf("Expected WindowWidth to be 120, got %d", updatedModel.WindowWidth)
	}

	if updatedModel.WindowHeight != 40 {
		t.Errorf("Expected WindowHeight to be 40, got %d", updatedModel.WindowHeight)
	}
}

func TestUpdate_UnknownKeyHandling(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)

	// Test unknown key in main mode
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on unknown key, got %v", cmd)
	}

	// Model should remain unchanged
	if newModel != model {
		t.Error("Expected model to remain unchanged on unknown key")
	}
}

func TestUpdate_UnknownMessageType(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)

	// Test unknown message type
	unknownMsg := tea.MouseMsg{}
	newModel, cmd := model.Update(unknownMsg)

	if cmd != nil {
		t.Errorf("Expected no command on unknown message type, got %v", cmd)
	}

	// Model should remain unchanged
	if newModel != model {
		t.Error("Expected model to remain unchanged on unknown message type")
	}
}

// Additional command palette tests as requested in the issue

func TestUpdate_SetCommand(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50, Environment: "Development", ApplicationInsightsKey: ""}
	model := InitialModel(cfg)
	model.CommandPalette = true

	testCases := []struct {
		name            string
		command         string
		expectedFetch   int
		expectedEnv     string
		expectedKey     string
		expectError     bool
		expectedMessage string
	}{
		{
			name:            "set fetchSize",
			command:         "set fetchSize=100",
			expectedFetch:   100,
			expectedEnv:     "Development",
			expectedKey:     "",
			expectError:     false,
			expectedMessage: "✓ fetchSize set to: 100",
		},
		{
			name:            "set environment",
			command:         "set environment=Testing",
			expectedFetch:   50,
			expectedEnv:     "Testing",
			expectedKey:     "",
			expectError:     false,
			expectedMessage: "✓ environment set to: Testing",
		},
		{
			name:            "set applicationInsightsKey",
			command:         "set applicationInsightsKey=test-key",
			expectedFetch:   50,
			expectedEnv:     "Development",
			expectedKey:     "test-key",
			expectError:     false,
			expectedMessage: "✓ applicationInsightsKey set to: test-key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset model for each test
			model = InitialModel(cfg)
			model.CommandPalette = true
			model.CommandInput = tc.command

			// Send enter key to execute command
			keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
			newModel, cmd := model.Update(keyMsg)

			if cmd != nil {
				t.Errorf("Expected no command on enter, got %v", cmd)
			}

			updatedModel := newModel.(Model)

			// Check command palette is closed
			if updatedModel.CommandPalette {
				t.Error("Expected CommandPalette to be false after command execution")
			}

			// Check config was updated
			if updatedModel.Config.LogFetchSize != tc.expectedFetch {
				t.Errorf("Expected LogFetchSize to be %d, got %d", tc.expectedFetch, updatedModel.Config.LogFetchSize)
			}
			if updatedModel.Config.Environment != tc.expectedEnv {
				t.Errorf("Expected Environment to be %q, got %q", tc.expectedEnv, updatedModel.Config.Environment)
			}
			if updatedModel.Config.ApplicationInsightsKey != tc.expectedKey {
				t.Errorf("Expected ApplicationInsightsKey to be %q, got %q", tc.expectedKey, updatedModel.Config.ApplicationInsightsKey)
			}

			// Check feedback message
			if tc.expectError {
				if !updatedModel.FeedbackIsError {
					t.Errorf("Expected error feedback for command %q", tc.command)
				}
			} else {
				if updatedModel.FeedbackIsError {
					t.Errorf("Expected success feedback for command %q, got error: %s", tc.command, updatedModel.FeedbackMessage)
				}
			}

			if !strings.Contains(updatedModel.FeedbackMessage, tc.expectedMessage) {
				t.Errorf("Expected feedback message to contain %q, got %q", tc.expectedMessage, updatedModel.FeedbackMessage)
			}
		})
	}
}

func TestUpdate_SetCommandList(t *testing.T) {
	cfg := config.Config{
		LogFetchSize:           99,
		Environment:            "ListTest",
		ApplicationInsightsKey: "list-key",
	}
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = "set"

	// Send enter key to execute "set" command
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on enter, got %v", cmd)
	}

	updatedModel := newModel.(Model)

	// Check command palette is closed
	if updatedModel.CommandPalette {
		t.Error("Expected CommandPalette to be false after command execution")
	}

	// Check feedback shows current settings
	if updatedModel.FeedbackIsError {
		t.Errorf("Expected success feedback for 'set' command, got error: %s", updatedModel.FeedbackMessage)
	}

	expectedSettings := []string{"fetchSize=99", "environment=ListTest", "applicationInsightsKey=***"}
	for _, expected := range expectedSettings {
		if !strings.Contains(updatedModel.FeedbackMessage, expected) {
			t.Errorf("Expected feedback message to contain %q, got %q", expected, updatedModel.FeedbackMessage)
		}
	}
}

func TestUpdate_SetCommandValidation(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true

	testCases := []struct {
		name            string
		command         string
		expectError     bool
		expectedMessage string
	}{
		{
			name:            "invalid fetchSize - negative",
			command:         "set fetchSize=-10",
			expectError:     true,
			expectedMessage: "fetchSize must be a positive integer",
		},
		{
			name:            "invalid fetchSize - zero",
			command:         "set fetchSize=0",
			expectError:     true,
			expectedMessage: "fetchSize must be a positive integer",
		},
		{
			name:            "invalid fetchSize - non-integer",
			command:         "set fetchSize=abc",
			expectError:     true,
			expectedMessage: "fetchSize must be a positive integer",
		},
		{
			name:            "invalid environment - empty",
			command:         "set environment=",
			expectError:     true,
			expectedMessage: "environment cannot be empty",
		},
		{
			name:            "unknown setting",
			command:         "set unknownSetting=value",
			expectError:     true,
			expectedMessage: "unknown setting: unknownSetting",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset model for each test
			model = InitialModel(cfg)
			model.CommandPalette = true
			model.CommandInput = tc.command

			// Send enter key to execute command
			keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
			newModel, cmd := model.Update(keyMsg)

			if cmd != nil {
				t.Errorf("Expected no command on enter, got %v", cmd)
			}

			updatedModel := newModel.(Model)

			// Check command palette is closed
			if updatedModel.CommandPalette {
				t.Error("Expected CommandPalette to be false after command execution")
			}

			// Check error feedback
			if tc.expectError {
				if !updatedModel.FeedbackIsError {
					t.Errorf("Expected error feedback for command %q", tc.command)
				}
				if !strings.Contains(updatedModel.FeedbackMessage, tc.expectedMessage) {
					t.Errorf("Expected error message to contain %q, got %q", tc.expectedMessage, updatedModel.FeedbackMessage)
				}
			}
		})
	}
}

func TestUpdate_SetCommandConfirmation(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true
	model.CommandInput = "set fetchSize=150"

	// Send enter key to execute command
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := model.Update(keyMsg)

	if cmd != nil {
		t.Errorf("Expected no command on enter, got %v", cmd)
	}

	updatedModel := newModel.(Model)

	// Check success confirmation
	if updatedModel.FeedbackIsError {
		t.Errorf("Expected success feedback, got error: %s", updatedModel.FeedbackMessage)
	}

	expectedConfirmation := "✓ fetchSize set to: 150"
	if !strings.Contains(updatedModel.FeedbackMessage, expectedConfirmation) {
		t.Errorf("Expected confirmation message %q, got %q", expectedConfirmation, updatedModel.FeedbackMessage)
	}

	// Check HelpText was updated for fetchSize
	expectedHelpText := "Log fetch size: 150"
	if !strings.Contains(updatedModel.HelpText, expectedHelpText) {
		t.Errorf("Expected HelpText to contain %q, got %q", expectedHelpText, updatedModel.HelpText)
	}
}

func TestUpdate_SetCommandError(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true

	testCases := []struct {
		name            string
		command         string
		expectedMessage string
	}{
		{
			name:            "malformed command - no equals",
			command:         "set fetchSize 100",
			expectedMessage: "Usage: set <setting>=<value> or just 'set' to list all settings",
		},
		{
			name:            "malformed command - multiple equals",
			command:         "set fetchSize=100=extra",
			expectedMessage: "fetchSize must be a positive integer", // Will try to parse "100=extra"
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset model for each test
			model = InitialModel(cfg)
			model.CommandPalette = true
			model.CommandInput = tc.command

			// Send enter key to execute command
			keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
			newModel, cmd := model.Update(keyMsg)

			if cmd != nil {
				t.Errorf("Expected no command on enter, got %v", cmd)
			}

			updatedModel := newModel.(Model)

			// Check error feedback
			if !updatedModel.FeedbackIsError {
				t.Errorf("Expected error feedback for malformed command %q", tc.command)
			}

			if !strings.Contains(updatedModel.FeedbackMessage, tc.expectedMessage) {
				t.Errorf("Expected error message to contain %q, got %q", tc.expectedMessage, updatedModel.FeedbackMessage)
			}
		})
	}
}

func TestUpdate_CommandParsing(t *testing.T) {
	cfg := config.Config{LogFetchSize: 50}
	model := InitialModel(cfg)
	model.CommandPalette = true

	testCases := []struct {
		name        string
		command     string
		expectError bool
		description string
	}{
		{
			name:        "unknown command",
			command:     "unknown command",
			expectError: true,
			description: "Should handle unknown commands gracefully",
		},
		{
			name:        "empty command",
			command:     "",
			expectError: false,
			description: "Should handle empty commands gracefully",
		},
		{
			name:        "whitespace command",
			command:     "   ",
			expectError: false,
			description: "Should handle whitespace-only commands gracefully",
		},
		{
			name:        "set with spaces",
			command:     "  set   fetchSize=200  ",
			expectError: false,
			description: "Should handle commands with extra whitespace",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset model for each test
			model = InitialModel(cfg)
			model.CommandPalette = true
			model.CommandInput = tc.command

			// Send enter key to execute command
			keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
			newModel, cmd := model.Update(keyMsg)

			if cmd != nil {
				t.Errorf("Expected no command on enter, got %v", cmd)
			}

			updatedModel := newModel.(Model)

			// Check command palette is closed regardless of command result
			if updatedModel.CommandPalette {
				t.Error("Expected CommandPalette to be false after command execution")
			}

			// Check input is cleared
			if updatedModel.CommandInput != "" {
				t.Errorf("Expected CommandInput to be cleared, got %q", updatedModel.CommandInput)
			}

			// Check error handling for unknown commands
			if tc.expectError {
				if !updatedModel.FeedbackIsError {
					t.Errorf("Expected error feedback for command %q", tc.command)
				}
				if strings.TrimSpace(tc.command) != "" && !strings.Contains(updatedModel.FeedbackMessage, "Unknown command") {
					t.Errorf("Expected 'Unknown command' error message, got %q", updatedModel.FeedbackMessage)
				}
			}
		})
	}
}
