package main

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMain_ConfigurationToTUIFlow(t *testing.T) {
	// Test the end-to-end flow from config loading to TUI initialization

	// Set test environment
	os.Setenv("LOG_FETCH_SIZE", "123")
	defer os.Unsetenv("LOG_FETCH_SIZE")

	// Load config
	cfg := config.LoadConfig()
	if cfg.LogFetchSize != 123 {
		t.Errorf("Expected config LogFetchSize to be 123, got %d", cfg.LogFetchSize)
	}

	// Initialize TUI with config
	model := tui.InitialModel(cfg)

	// Verify config flows through to TUI
	if model.Config.LogFetchSize != 123 {
		t.Errorf("Expected TUI model LogFetchSize to be 123, got %d", model.Config.LogFetchSize)
	}

	// Verify config is reflected in help text
	view := model.View()
	if !contains(view, "Log fetch size: 123") {
		t.Errorf("Expected view to contain log fetch size 123, got: %s", view)
	}
}

func TestMain_TUIInitialization(t *testing.T) {
	// Test TUI can be initialized without errors
	cfg := config.LoadConfig()
	model := tui.InitialModel(cfg)

	// Create a Bubble Tea program (but don't run it)
	program := tea.NewProgram(model, tea.WithoutRenderer())

	if program == nil {
		t.Error("Expected NewProgram to return a valid program")
	}

	// Test initial command
	cmd := model.Init()
	if cmd != nil {
		t.Errorf("Expected Init() to return nil, got %v", cmd)
	}
}

func TestMain_TUIBasicInteraction(t *testing.T) {
	// Test basic TUI interactions
	cfg := config.Config{LogFetchSize: 50}
	model := tui.InitialModel(cfg)

	// Test quit command
	quitKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	_, cmd := model.Update(quitKey)

	if cmd == nil {
		t.Errorf("Expected quit command to return quit function, got nil")
	}

	// Test command palette
	ctrlPKey := tea.KeyMsg{Type: tea.KeyCtrlP}
	newModel, cmd := model.Update(ctrlPKey)

	if cmd != nil {
		t.Errorf("Expected Ctrl+P to return nil command, got %v", cmd)
	}

	updatedModel := newModel.(tui.Model)
	if !updatedModel.CommandPalette {
		t.Error("Expected CommandPalette to be true after Ctrl+P")
	}
}

func TestMain_ConfigurationVariations(t *testing.T) {
	// Test different configuration scenarios
	testCases := []struct {
		name     string
		envVar   string
		expected int
	}{
		{"no env var", "", 50},
		{"valid env var", "200", 200},
		{"invalid env var", "invalid", 50},
		{"zero env var", "0", 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment
			if tc.envVar != "" {
				os.Setenv("LOG_FETCH_SIZE", tc.envVar)
				defer os.Unsetenv("LOG_FETCH_SIZE")
			} else {
				os.Unsetenv("LOG_FETCH_SIZE")
			}

			// Test full flow
			cfg := config.LoadConfig()
			model := tui.InitialModel(cfg)

			if model.Config.LogFetchSize != tc.expected {
				t.Errorf("Expected LogFetchSize to be %d, got %d",
					tc.expected, model.Config.LogFetchSize)
			}

			// Verify it appears in the view
			view := model.View()
			expectedText := "Log fetch size: " + itoa(tc.expected)
			if !contains(view, expectedText) {
				t.Errorf("Expected view to contain %q, got: %s", expectedText, view)
			}
		})
	}
}

func TestMain_TerminalResizeIntegration(t *testing.T) {
	// Test terminal resize handling in full context
	cfg := config.LoadConfig()
	model := tui.InitialModel(cfg)

	// Test initial size
	if model.WindowWidth != 80 || model.WindowHeight != 24 {
		t.Errorf("Expected initial size 80x24, got %dx%d", model.WindowWidth, model.WindowHeight)
	}

	// Test resize
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	newModel, cmd := model.Update(resizeMsg)

	if cmd != nil {
		t.Errorf("Expected resize to return nil command, got %v", cmd)
	}

	updatedModel := newModel.(tui.Model)
	if updatedModel.WindowWidth != 100 || updatedModel.WindowHeight != 30 {
		t.Errorf("Expected resized size 100x30, got %dx%d",
			updatedModel.WindowWidth, updatedModel.WindowHeight)
	}

	// Verify size appears in view
	view := updatedModel.View()
	if !contains(view, "Terminal size: 100x30") {
		t.Errorf("Expected view to contain terminal size, got: %s", view)
	}
}

func TestMain_ExecutableBuild(t *testing.T) {
	// Test that the application can be built as an executable
	cmd := exec.Command("go", "build", "-o", "/tmp/bc-insights-tui-test")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build executable: %v\nOutput: %s", err, output)
	}

	// Verify the executable exists and is executable
	execPath := "/tmp/bc-insights-tui-test"
	info, err := os.Stat(execPath)
	if err != nil {
		t.Fatalf("Built executable not found: %v", err)
	}

	// Check it's executable
	if info.Mode()&0o111 == 0 {
		t.Error("Built file is not executable")
	}

	// Clean up
	defer os.Remove(execPath)
}

func TestMain_ExecutableRun(t *testing.T) {
	// Test that the built executable can start and respond to input
	// This test is more fragile but tests the actual end-user experience

	// Build the executable
	buildCmd := exec.Command("go", "build", "-o", "/tmp/bc-insights-tui-test")
	buildCmd.Dir = "."

	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build executable for testing: %v", err)
	}
	defer os.Remove("/tmp/bc-insights-tui-test")

	// Run the executable with a timeout and quit command
	runCmd := exec.Command("/tmp/bc-insights-tui-test")

	// Use a timeout to prevent hanging
	timeout := time.After(5 * time.Second)
	done := make(chan error, 1)

	go func() {
		done <- runCmd.Run()
	}()

	// Send quit signal after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		if runCmd.Process != nil {
			runCmd.Process.Signal(os.Interrupt)
		}
	}()

	select {
	case err := <-done:
		// Process exited, this is expected with interrupt signal
		if err != nil {
			// Check if it's a signal-related exit or TUI-related exit (expected)
			if exitError, ok := err.(*exec.ExitError); ok {
				// Exit codes 1, 2, or 130 typically indicate interrupt or TUI issues
				if exitError.ExitCode() != 130 && exitError.ExitCode() != 2 && exitError.ExitCode() != 0 && exitError.ExitCode() != 1 {
					t.Errorf("Unexpected exit code: %d", exitError.ExitCode())
				}
				// Exit code 1 is acceptable for TUI apps without proper terminal
			}
		}
	case <-timeout:
		// Kill the process if it's still running
		if runCmd.Process != nil {
			runCmd.Process.Kill()
		}
		t.Error("Executable did not exit within timeout")
	}
}

func TestMain_GracefulShutdown(t *testing.T) {
	// Test that the TUI handles shutdown scenarios properly
	cfg := config.LoadConfig()
	model := tui.InitialModel(cfg)

	// Test quit via 'q'
	quitKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	_, cmd := model.Update(quitKey)

	if cmd == nil {
		t.Errorf("Expected 'q' to trigger graceful shutdown, got nil")
	}

	// Test quit via Ctrl+C
	ctrlCKey := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd = model.Update(ctrlCKey)

	if cmd == nil {
		t.Errorf("Expected Ctrl+C to trigger graceful shutdown, got nil")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte(n%10) + '0'}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
