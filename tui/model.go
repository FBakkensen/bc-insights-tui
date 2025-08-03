package tui

// Model for Bubble Tea TUI

import (
	"fmt"

	"github.com/FBakkensen/bc-insights-tui/config"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	WelcomeMsg string
	HelpText   string
	Config     config.Config
}

func InitialModel(cfg config.Config) Model {
	return Model{
		WelcomeMsg: "Welcome to bc-insights-tui!",
		HelpText:   fmt.Sprintf("Press q to quit. Log fetch size: %d", cfg.LogFetchSize),
		Config:     cfg,
	}
}

// Init implements tea.Model interface
func (m Model) Init() tea.Cmd {
	return nil
}
