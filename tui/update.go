package tui

// Update logic for Bubble Tea TUI

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle command palette input first
		if m.CommandPalette {
			switch msg.String() {
			case "esc":
				// Close command palette
				m.CommandPalette = false
				m.CommandInput = ""
				return m, nil
			case "enter":
				// Process command (placeholder for future implementation)
				m.CommandPalette = false
				m.CommandInput = ""
				return m, nil
			case "backspace":
				// Remove last character from command input
				if len(m.CommandInput) > 0 {
					m.CommandInput = m.CommandInput[:len(m.CommandInput)-1]
				}
				return m, nil
			default:
				// Add character to command input
				if len(msg.String()) == 1 {
					m.CommandInput += msg.String()
				}
				return m, nil
			}
		}

		// Handle main application input
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+p":
			// Open command palette
			m.CommandPalette = true
			m.CommandInput = ""
			return m, nil
		case "esc":
			// Close any open modals (currently just command palette)
			if m.CommandPalette {
				m.CommandPalette = false
				m.CommandInput = ""
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		// Handle terminal resize
		m.WindowWidth = msg.Width
		m.WindowHeight = msg.Height
		return m, nil
	}
	return m, nil
}
