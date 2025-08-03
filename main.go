package main

// Entry point for bc-insights-tui
import (
	"fmt"
	"os"

	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize the TUI with configuration
	p := tea.NewProgram(tui.InitialModel(cfg))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
