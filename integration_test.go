package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMain_ConfigurationToTUIFlow(t *testing.T) {
	// Test the end-to-end flow from config loading to TUI initialization

	// Set test environment
	if err := os.Setenv("LOG_FETCH_SIZE", "123"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
			t.Logf("Failed to unset environment variable: %v", err)
		}
	}()

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
	if !strings.Contains(view, "Log fetch size: 123") {
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
				if err := os.Setenv("LOG_FETCH_SIZE", tc.envVar); err != nil {
					t.Fatalf("Failed to set environment variable: %v", err)
				}
				defer func() {
					if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
						t.Logf("Failed to unset environment variable: %v", err)
					}
				}()
			} else {
				if err := os.Unsetenv("LOG_FETCH_SIZE"); err != nil {
					t.Logf("Failed to unset environment variable: %v", err)
				}
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
			expectedText := "Log fetch size: " + strconv.Itoa(tc.expected)
			if !strings.Contains(view, expectedText) {
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
	if !strings.Contains(view, "Terminal: 100x30") {
		t.Errorf("Expected view to contain terminal size, got: %s", view)
	}
}

func TestMain_ExecutableBuild(t *testing.T) {
	// Test that the application can be built as an executable
	tempDir := os.TempDir()
	execPath := tempDir + string(os.PathSeparator) + "bc-insights-tui-test.exe"
	cmd := exec.Command("go", "build", "-o", execPath)
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build executable: %v\nOutput: %s", err, output)
	}

	// Verify the executable exists and is executable
	info, err := os.Stat(execPath)
	if err != nil {
		t.Fatalf("Built executable not found: %v", err)
	}

	// Check it's executable (on Windows, this is less relevant but we can check it exists)
	if info.Size() == 0 {
		t.Error("Built file has zero size")
	}

	// Clean up
	defer func() {
		if err := os.Remove(execPath); err != nil {
			t.Logf("Failed to remove test executable: %v", err)
		}
	}()
}

func TestMain_ExecutableRun(t *testing.T) {
	// Test that the built executable can start and respond to input
	// This test verifies the executable can be launched but may hang in test environment
	// which is expected behavior for TUI applications without proper terminal

	// Build the executable
	tempDir := os.TempDir()
	execPath := tempDir + string(os.PathSeparator) + "bc-insights-tui-test.exe"
	buildCmd := exec.Command("go", "build", "-o", execPath)
	buildCmd.Dir = "."

	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build executable for testing: %v", err)
	}
	defer func() {
		// Give Windows time to release the file handle
		time.Sleep(200 * time.Millisecond)
		if err := os.Remove(execPath); err != nil {
			t.Logf("Failed to remove test executable: %v", err)
		}
	}()

	// Run the executable with a short timeout
	// TUI apps typically hang in test environments without proper terminal
	runCmd := exec.Command(execPath)

	// Start the process
	if err := runCmd.Start(); err != nil {
		t.Fatalf("Failed to start executable: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Kill the process - TUI apps can't run properly in test environment
	if runCmd.Process != nil {
		if err := runCmd.Process.Kill(); err != nil {
			t.Logf("Failed to kill process: %v", err)
		}
	}

	// Wait for the process to complete
	err := runCmd.Wait()
	if err != nil {
		t.Logf("Executable exited with error (expected): %v", err)
	} else {
		t.Log("Executable exited cleanly")
	}

	// The fact that we can build and start the executable is the main success criteria
	// Hanging in test environment is expected behavior for TUI applications
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
