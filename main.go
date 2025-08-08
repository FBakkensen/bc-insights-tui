package main

// Entry point for bc-insights-tui
import (
	"fmt"

	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/logging"
	"github.com/FBakkensen/bc-insights-tui/tui"
)

func main() {
	// Initialize logging first
	logLevel := logging.LevelInfo
	if err := logging.InitLogger(logLevel); err != nil {
		fmt.Printf("Warning: Failed to initialize logging: %v\n", err)
	}
	defer logging.Close()

	logging.Info("Starting bc-insights-tui application (chat-first rewrite - Step 1)")

	// Load configuration
	cfg := config.LoadConfig()
	logging.Info("Configuration loaded", "logFetchSize", fmt.Sprintf("%d", cfg.LogFetchSize))

	// Start chat-first UI (Step 1: Login)
	if err := tui.Run(cfg); err != nil {
		logging.Error("UI exited with error", "error", err.Error())
		fmt.Println("Error:", err)
	}
}
