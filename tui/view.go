package tui

// View logic for Bubble Tea TUI

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	var s strings.Builder

	// Main application view
	s.WriteString(m.WelcomeMsg + "\n\n")
	s.WriteString(m.HelpText + "\n")

	// Terminal size info (for debugging/info)
	s.WriteString(fmt.Sprintf("\nTerminal size: %dx%d\n", m.WindowWidth, m.WindowHeight))

	// Command palette overlay
	if m.CommandPalette {
		s.WriteString("\n" + strings.Repeat("─", min(m.WindowWidth, 50)) + "\n")
		s.WriteString("Command Palette (press Esc to close, Enter to execute):\n")
		s.WriteString("> " + m.CommandInput + "\n")
		s.WriteString(strings.Repeat("─", min(m.WindowWidth, 50)) + "\n")
	}

	return s.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
