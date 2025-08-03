package tui

// View logic for Bubble Tea TUI

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	// Define styles for the full-screen layout
	var (
		// Header style - top border bar
		headerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("24")).
				Bold(true).
				Padding(0, 1).
				Width(m.WindowWidth)

		// Main content container style
		contentStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("24")).
				Padding(1, 2).
				Width(m.WindowWidth - 4).  // Account for borders
				Height(m.WindowHeight - 4) // Account for header and footer

		// Footer style - bottom border bar
		footerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("24")).
				Padding(0, 1).
				Width(m.WindowWidth)

		// Title styles
		titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true)

		// Status styles
		statusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

		// Section header styles
		sectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Bold(true)

		// Bullet point styles
		bulletStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("33"))
	)

	// Build header content
	headerLeft := fmt.Sprintf("bc-insights-tui %s", AppVersion)
	headerRight := "Welcome - Business Central Developer Tools"
	headerSpacer := strings.Repeat(" ", max(0, m.WindowWidth-len(headerLeft)-len(headerRight)-7))
	header := headerStyle.Render(headerLeft + " | " + headerRight + headerSpacer)

	// Build main content
	var content strings.Builder

	// Welcome section
	content.WriteString(titleStyle.Render("Welcome to bc-insights-tui!"))
	content.WriteString("\n")
	content.WriteString("Terminal User Interface for Azure Application Insights\n\n")

	content.WriteString("Built specifically for Microsoft Dynamics 365 Business Central developers\n")
	content.WriteString("to analyze telemetry data with an AI-powered, command palette-driven workflow.\n\n")

	// Status section
	content.WriteString(statusStyle.Render("🔧 Current Status: Not authenticated (authentication required)"))
	content.WriteString("\n\n")

	// Keyboard shortcuts section
	content.WriteString(sectionStyle.Render("📋 Essential Keyboard Shortcuts:"))
	content.WriteString("\n")
	content.WriteString(bulletStyle.Render("• Ctrl+P") + "  - Open command palette (your primary tool)\n")
	content.WriteString(bulletStyle.Render("• Ctrl+C") + "  - Quit application\n")
	content.WriteString(bulletStyle.Render("• q") + "       - Quit application\n")
	content.WriteString(bulletStyle.Render("• Esc") + "     - Cancel operations\n\n")

	// AI features preview section
	content.WriteString(sectionStyle.Render("🤖 Coming Soon: AI-powered KQL query generation (Phase 5)"))
	content.WriteString("\n")
	content.WriteString("     Use natural language to generate complex queries: \"ai: show all errors from today\"\n\n")

	// Configuration section
	content.WriteString(sectionStyle.Render("⚙️  Configuration:"))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf(bulletStyle.Render("• Log fetch size: %d entries per request"), m.Config.LogFetchSize))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf(bulletStyle.Render("• Environment: %s"), m.Config.Environment))
	content.WriteString("\n")
	if m.Config.ApplicationInsightsKey == "" {
		content.WriteString(bulletStyle.Render("• Application Insights Key: (not set)"))
	} else {
		masked := m.Config.ApplicationInsightsKey
		if len(masked) > 8 {
			masked = masked[:4] + "..." + masked[len(masked)-4:]
		} else {
			masked = "***"
		}
		content.WriteString(fmt.Sprintf(bulletStyle.Render("• Application Insights Key: %s"), masked))
	}
	content.WriteString("\n")

	// Command feedback section
	if m.FeedbackMessage != "" {
		content.WriteString("\n")
		if m.FeedbackIsError {
			errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
			content.WriteString(errorStyle.Render("❌ " + m.FeedbackMessage))
		} else {
			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
			content.WriteString(successStyle.Render(m.FeedbackMessage))
		}
		content.WriteString("\n")
	}

	// Apply content styling
	mainContent := contentStyle.Render(content.String())

	// Build footer content
	footerLeft := "[Ctrl+P] Open Command Palette | [Ctrl+C] Quit"
	footerRight := fmt.Sprintf("Terminal: %dx%d", m.WindowWidth, m.WindowHeight)
	footerSpacer := strings.Repeat(" ", max(0, m.WindowWidth-len(footerLeft)-len(footerRight)-4))
	footer := footerStyle.Render(footerLeft + footerSpacer + footerRight)

	// Combine all parts for full screen layout
	var screen strings.Builder
	screen.WriteString(header)
	screen.WriteString("\n")
	screen.WriteString(mainContent)
	screen.WriteString("\n")
	screen.WriteString(footer)

	// Handle command palette overlay
	if m.CommandPalette {
		// Calculate overlay position (center of screen)
		overlayWidth := min(60, m.WindowWidth-10)
		overlayHeight := 5

		paletteStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("226")).
			Background(lipgloss.Color("235")).
			Padding(1, 2).
			Width(overlayWidth).
			Align(lipgloss.Center)

		var paletteContent strings.Builder
		paletteContent.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")).Render("Command Palette"))
		paletteContent.WriteString("\n")
		paletteContent.WriteString("Examples: 'set' to list settings, 'set fetchSize=100'\n")
		paletteContent.WriteString("Press Esc to close, Enter to execute\n\n")
		paletteContent.WriteString("> " + m.CommandInput)

		palette := paletteStyle.Render(paletteContent.String())

		// Position overlay in the center of the screen
		lines := strings.Split(screen.String(), "\n")
		if len(lines) > overlayHeight+2 {
			startLine := (len(lines) - overlayHeight) / 2
			paletteLines := strings.Split(palette, "\n")
			for i, paletteLine := range paletteLines {
				if startLine+i < len(lines) {
					lines[startLine+i] = paletteLine
				}
			}
			screen.Reset()
			screen.WriteString(strings.Join(lines, "\n"))
		} else {
			// Fallback if screen is too small
			screen.WriteString("\n")
			screen.WriteString(palette)
		}
	}

	return screen.String() + "\n"
}
