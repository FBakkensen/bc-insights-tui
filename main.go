package main

// Entry point for bc-insights-tui
import (
	"fmt"
	"os"

	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/logging"
)

func main() {
	// Initialize logging first
	logLevel := os.Getenv("BC_INSIGHTS_LOG_LEVEL")
	if logLevel == "" {
		logLevel = logging.LevelInfo // Default to INFO level
	}

	if err := logging.InitLogger(logLevel); err != nil {
		fmt.Printf("Warning: Failed to initialize logging: %v\n", err)
	}
	defer logging.Close()

	logging.Info("Starting bc-insights-tui application (chat-first rewrite - Step 0)")

	// Load configuration
	cfg := config.LoadConfig()
	logging.Info("Configuration loaded", "logFetchSize", fmt.Sprintf("%d", cfg.LogFetchSize))

	// Step 0: TUI removed. Provide a simple placeholder until chat-first UI is implemented.
	fmt.Println("bc-insights-tui: UI removed (Step 0). Chat-first rewrite in progress.")
	fmt.Println("Configuration loaded successfully. Nothing to run yet.")
	logging.Info("Exiting after Step 0 placeholder")
}
