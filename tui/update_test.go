package tui

import (
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
