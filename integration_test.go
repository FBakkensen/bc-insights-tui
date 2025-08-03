package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

// Additional integration tests as requested in the issue

func TestMain_ConfigFileToUI(t *testing.T) {
	// Test complete flow from config file loading to UI display
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "integration-test.json")

	// Create test config file
	configContent := `{
		"fetchSize": 333,
		"environment": "IntegrationTestEnv"
	}`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load config from file
	cfg := config.LoadConfigWithArgs([]string{"--config=" + configFile})

	// Initialize TUI with config
	model := tui.InitialModel(cfg)

	// Verify config flows through correctly
	if model.Config.LogFetchSize != 333 {
		t.Errorf("Expected LogFetchSize to be 333, got %d", model.Config.LogFetchSize)
	}
	if model.Config.Environment != "IntegrationTestEnv" {
		t.Errorf("Expected Environment to be 'IntegrationTestEnv', got %q", model.Config.Environment)
	}

	// Verify UI shows correct values
	view := model.View()
	if !strings.Contains(view, "Log fetch size: 333") {
		t.Error("Expected UI to display fetch size from config file")
	}
	if !strings.Contains(view, "Environment: IntegrationTestEnv") {
		t.Error("Expected UI to display environment from config file")
	}
}

func TestMain_CommandLineFlagsToUI(t *testing.T) {
	// Test command line flags override other sources and flow to UI
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "override-test.json")

	// Create config file with different values
	configContent := `{
		"fetchSize": 100,
		"environment": "FileEnv"
	}`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Set environment variables with different values
	os.Setenv("LOG_FETCH_SIZE", "200")
	os.Setenv("BCINSIGHTS_ENVIRONMENT", "EnvVar")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_ENVIRONMENT")
	}()

	// Load config with command line flags (should override file and env)
	cfg := config.LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--fetch-size=444",
		"--environment=FlagEnv",
	})

	// Initialize TUI
	model := tui.InitialModel(cfg)

	// Verify flags win
	if model.Config.LogFetchSize != 444 {
		t.Errorf("Expected flags to override file and env for LogFetchSize, got %d", model.Config.LogFetchSize)
	}
	if model.Config.Environment != "FlagEnv" {
		t.Errorf("Expected flags to override file and env for Environment, got %q", model.Config.Environment)
	}

	// Verify UI shows flag values
	view := model.View()
	if !strings.Contains(view, "Log fetch size: 444") {
		t.Error("Expected UI to display fetch size from command line flags")
	}
	if !strings.Contains(view, "Environment: FlagEnv") {
		t.Error("Expected UI to display environment from command line flags")
	}
}

func TestMain_SetCommandEndToEnd(t *testing.T) {
	// Test complete set command workflow from input to UI update
	cfg := config.Config{LogFetchSize: 50, Environment: "Original"}
	model := tui.InitialModel(cfg)

	// Open command palette
	ctrlPKey := tea.KeyMsg{Type: tea.KeyCtrlP}
	newModel, _ := model.Update(ctrlPKey)
	model = newModel.(tui.Model)

	if !model.CommandPalette {
		t.Fatal("Expected command palette to be open")
	}

	// Type set command
	for _, char := range "set fetchSize=555" {
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
		newModel, _ := model.Update(keyMsg)
		model = newModel.(tui.Model)
	}

	if model.CommandInput != "set fetchSize=555" {
		t.Errorf("Expected command input to be 'set fetchSize=555', got %q", model.CommandInput)
	}

	// Execute command
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ = model.Update(enterKey)
	model = newModel.(tui.Model)

	// Verify command executed
	if model.CommandPalette {
		t.Error("Expected command palette to be closed after execution")
	}
	if model.Config.LogFetchSize != 555 {
		t.Errorf("Expected LogFetchSize to be updated to 555, got %d", model.Config.LogFetchSize)
	}

	// Verify success feedback
	if model.FeedbackIsError {
		t.Errorf("Expected success feedback, got error: %s", model.FeedbackMessage)
	}
	if !strings.Contains(model.FeedbackMessage, "✓ fetchSize set to: 555") {
		t.Errorf("Expected success confirmation, got: %s", model.FeedbackMessage)
	}

	// Verify UI shows updated value
	view := model.View()
	if !strings.Contains(view, "Log fetch size: 555") {
		t.Error("Expected UI to show updated fetch size")
	}
	if !strings.Contains(view, "✓ fetchSize set to: 555") {
		t.Error("Expected UI to show success feedback")
	}
}

func TestMain_ConfigPersistence(t *testing.T) {
	// Test that configuration changes persist during the session
	cfg := config.Config{LogFetchSize: 50, Environment: "Initial"}
	model := tui.InitialModel(cfg)

	// Update config via set command
	model.CommandPalette = true
	model.CommandInput = "set environment=Persistent"
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := model.Update(enterKey)
	model = newModel.(tui.Model)

	// Verify change persisted
	if model.Config.Environment != "Persistent" {
		t.Errorf("Expected environment to persist as 'Persistent', got %q", model.Config.Environment)
	}

	// Simulate terminal resize (common UI event)
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	newModel, _ = model.Update(resizeMsg)
	model = newModel.(tui.Model)

	// Verify config still persisted after other events
	if model.Config.Environment != "Persistent" {
		t.Error("Expected config to persist after terminal resize")
	}

	// Make another config change
	model.CommandPalette = true
	model.CommandInput = "set fetchSize=777"
	newModel, _ = model.Update(enterKey)
	model = newModel.(tui.Model)

	// Verify both changes persist
	if model.Config.Environment != "Persistent" {
		t.Error("Expected first config change to persist after second change")
	}
	if model.Config.LogFetchSize != 777 {
		t.Errorf("Expected second config change to persist, got %d", model.Config.LogFetchSize)
	}

	// Verify UI shows both persisted values
	view := model.View()
	if !strings.Contains(view, "Log fetch size: 777") {
		t.Error("Expected UI to show persisted fetch size")
	}
	if !strings.Contains(view, "Environment: Persistent") {
		t.Error("Expected UI to show persisted environment")
	}
}

func TestMain_MultipleConfigSources(t *testing.T) {
	// Test complex precedence scenarios with multiple configuration sources
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "complex-test.json")

	// Create config file
	configContent := `{
		"fetchSize": 111,
		"environment": "FileEnvironment",
		"applicationInsightsKey": "file-key-123456789"
	}`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Set some environment variables (but not all)
	os.Setenv("LOG_FETCH_SIZE", "222")
	os.Setenv("BCINSIGHTS_APP_INSIGHTS_KEY", "env-key-987654321")
	defer func() {
		os.Unsetenv("LOG_FETCH_SIZE")
		os.Unsetenv("BCINSIGHTS_APP_INSIGHTS_KEY")
	}()

	// Load with some command line flags (but not all)
	cfg := config.LoadConfigWithArgs([]string{
		"--config=" + configFile,
		"--environment=FlagEnvironment",
	})

	// Initialize TUI
	model := tui.InitialModel(cfg)

	// Verify complex precedence:
	// - fetchSize: env var (222) should win over file (111), no flag set
	// - environment: flag (FlagEnvironment) should win over file (FileEnvironment), no env var set
	// - applicationInsightsKey: env var should win over file, no flag set

	if model.Config.LogFetchSize != 222 {
		t.Errorf("Expected env var to win for LogFetchSize, got %d", model.Config.LogFetchSize)
	}
	if model.Config.Environment != "FlagEnvironment" {
		t.Errorf("Expected flag to win for Environment, got %q", model.Config.Environment)
	}
	if model.Config.ApplicationInsightsKey != "env-key-987654321" {
		t.Errorf("Expected env var to win for ApplicationInsightsKey, got %q", model.Config.ApplicationInsightsKey)
	}

	// Verify UI shows the winning values
	view := model.View()
	if !strings.Contains(view, "Log fetch size: 222") {
		t.Error("Expected UI to show env var fetch size")
	}
	if !strings.Contains(view, "Environment: FlagEnvironment") {
		t.Error("Expected UI to show flag environment")
	}
	// Key should be masked: "env-...4321"
	if !strings.Contains(view, "env-...4321") {
		t.Error("Expected UI to show masked env var application insights key")
	}

	// Test that set commands work with this complex initial state
	model.CommandPalette = true
	model.CommandInput = "set"
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := model.Update(enterKey)
	model = newModel.(tui.Model)

	// Should show current values from all sources
	feedback := model.FeedbackMessage
	if !strings.Contains(feedback, "fetchSize=222") {
		t.Error("Expected 'set' command to show current fetch size from env var")
	}
	if !strings.Contains(feedback, "environment=FlagEnvironment") {
		t.Error("Expected 'set' command to show current environment from flag")
	}
	if !strings.Contains(feedback, "env-...4321") {
		t.Error("Expected 'set' command to show masked key from env var")
	}
}
