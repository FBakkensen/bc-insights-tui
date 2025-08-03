package tui

// View logic for Bubble Tea TUI

func (m Model) View() string {
	return m.WelcomeMsg + "\n\n" + m.HelpText + "\n"
}
