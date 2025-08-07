package tui

// View logic for Bubble Tea TUI

import (
	"fmt"
	"strings"

	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	// Handle authentication views
	switch m.AuthState {
	case auth.AuthStateRequired:
		return m.renderAuthRequiredView()
	case auth.AuthStateInProgress:
		return m.renderDeviceCodeView()
	case auth.AuthStateFailed:
		return m.renderAuthFailedView()
	case auth.AuthStateCompleted:
		// Continue to main view
		if m.KQLEditorMode {
			return m.renderKQLEditorView()
		}
	case auth.AuthStateUnknown:
		// Continue to main view (for now)
	}

	return m.renderMainView()
}

func (m Model) renderAuthRequiredView() string {
	var (
		titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true).
				Align(lipgloss.Center)

		contentStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("24")).
				Padding(2, 4).
				Width(60).
				Align(lipgloss.Center)

		instructionStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("214")).
					Bold(true)
	)

	var content strings.Builder
	content.WriteString(titleStyle.Render("🔐 Authentication Required"))
	content.WriteString("\n\n")
	content.WriteString("Welcome to bc-insights-tui!\n\n")
	content.WriteString("You need to authenticate with your Azure account to access\n")
	content.WriteString("Application Insights data.\n\n")
	content.WriteString(instructionStyle.Render("Press any key to start the authentication process..."))
	content.WriteString("\n\n")
	content.WriteString("Or press Ctrl+Q to quit.")

	box := contentStyle.Render(content.String())

	// Center the box on screen
	return lipgloss.Place(m.WindowWidth, m.WindowHeight, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderDeviceCodeView() string {
	var (
		titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true).
				Align(lipgloss.Center)

		contentStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("226")).
				Padding(2, 4).
				Width(70).
				Align(lipgloss.Center)

		codeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Bold(true).
				Background(lipgloss.Color("235")).
				Padding(0, 1)

		urlStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("33")).
				Underline(true)

		spinnerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))
	)

	var content strings.Builder
	content.WriteString(titleStyle.Render("🔐 Azure Authentication"))
	content.WriteString("\n\n")

	if m.DeviceCode != nil {
		content.WriteString("1. Open your web browser and go to:\n")
		content.WriteString("   " + urlStyle.Render(m.DeviceCode.VerificationURI))
		content.WriteString("\n\n")
		content.WriteString("2. Enter this device code:\n")
		content.WriteString("   " + codeStyle.Render(m.DeviceCode.UserCode))
		content.WriteString("\n\n")
		content.WriteString("3. Complete the sign-in process in your browser\n\n")
		content.WriteString(spinnerStyle.Render("⏳ Waiting for authentication to complete..."))
		content.WriteString("\n\n")
		content.WriteString(fmt.Sprintf("Code expires in %d minutes", m.DeviceCode.ExpiresIn/60))
	} else {
		content.WriteString(spinnerStyle.Render("⏳ Preparing authentication..."))
	}

	content.WriteString("\n\n")
	content.WriteString("Press Ctrl+Q to cancel and quit.")

	box := contentStyle.Render(content.String())

	// Center the box on screen
	return lipgloss.Place(m.WindowWidth, m.WindowHeight, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderAuthFailedView() string {
	var (
		titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Bold(true).
				Align(lipgloss.Center)

		contentStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("9")).
				Padding(2, 4).
				Width(70).
				Align(lipgloss.Center)

		errorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9"))

		instructionStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("214")).
					Bold(true)
	)

	var content strings.Builder
	content.WriteString(titleStyle.Render("❌ Authentication Failed"))
	content.WriteString("\n\n")

	if m.AuthError != nil {
		errorMsg := m.AuthError.Error()
		if strings.Contains(errorMsg, "authorization_declined") {
			content.WriteString("Authentication was declined in the browser.\n\n")
			content.WriteString("Please restart the login process and authorize the application.")
		} else if strings.Contains(errorMsg, "expired_token") {
			content.WriteString("The device code has expired.\n\n")
			content.WriteString("Please restart the authentication process.")
		} else if strings.Contains(errorMsg, "invalid_client") {
			content.WriteString("Configuration error: Invalid client credentials.\n\n")
			content.WriteString("Please check your Azure configuration and try again.")
		} else {
			content.WriteString("Authentication failed:\n\n")
			content.WriteString(errorStyle.Render(errorMsg))
		}
	} else {
		content.WriteString("Authentication failed due to an unknown error.")
	}

	content.WriteString("\n\n")
	content.WriteString(instructionStyle.Render("Press 'r' to retry authentication"))
	content.WriteString("\n")
	content.WriteString("Press Ctrl+Q to quit")

	box := contentStyle.Render(content.String())

	// Center the box on screen
	return lipgloss.Place(m.WindowWidth, m.WindowHeight, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderMainView() string {
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

	// Status section based on authentication state
	authStatus := "✅ Authenticated and ready!"
	if m.AuthState != auth.AuthStateCompleted {
		authStatus = "🔧 Authentication required"
	}
	content.WriteString(statusStyle.Render("Status: " + authStatus))
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

	// OAuth2 configuration
	content.WriteString(fmt.Sprintf(bulletStyle.Render("• Azure Tenant ID: %s"), m.Config.OAuth2.TenantID))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf(bulletStyle.Render("• Azure Client ID: %s"), m.Config.OAuth2.ClientID))
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
		paletteContent.WriteString("Examples: 'set' to list settings, 'set fetchSize=100', 'set environment=prod', 'set applicationInsightsKey=YOUR_KEY'\n")
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

// renderKQLEditorView renders the KQL editor interface
func (m Model) renderKQLEditorView() string {
	var (
		headerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("24")).
				Bold(true).
				Padding(0, 1).
				Width(m.WindowWidth)

		footerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("24")).
				Padding(0, 1).
				Width(m.WindowWidth)
	)

	// Build header
	headerLeft := fmt.Sprintf("bc-insights-tui %s - KQL Editor", AppVersion)
	headerRight := "F5: Execute | Tab: Switch Focus | Esc: Exit"
	headerSpacer := strings.Repeat(" ", max(0, m.WindowWidth-len(headerLeft)-len(headerRight)-3))
	header := headerStyle.Render(headerLeft + " | " + headerRight + headerSpacer)

	// Calculate component sizes
	totalHeight := m.WindowHeight - 2 // Header and footer
	editorHeight := int(float32(totalHeight-2) * m.Config.EditorPanelRatio)
	resultsHeight := totalHeight - editorHeight - 2

	// Ensure minimum heights
	if editorHeight < 5 {
		editorHeight = 5
	}
	if resultsHeight < 5 {
		resultsHeight = 5
	}

	// Update component sizes if needed
	m.KQLEditor.SetSize(m.WindowWidth, editorHeight)
	m.ResultsTable.SetSize(m.WindowWidth, resultsHeight)

	// Render components
	editorView := m.KQLEditor.View()
	resultsView := m.ResultsTable.View()

	// Build footer
	footerLeft := "[F5] Execute Query | [Tab] Switch Focus | [Esc] Exit Editor"
	footerRight := fmt.Sprintf("Focus: %s", strings.Title(m.FocusedComponent))
	footerSpacer := strings.Repeat(" ", max(0, m.WindowWidth-len(footerLeft)-len(footerRight)-4))
	footer := footerStyle.Render(footerLeft + footerSpacer + footerRight)

	// Combine all parts
	var screen strings.Builder
	screen.WriteString(header)
	screen.WriteString("\n")
	screen.WriteString(editorView)
	screen.WriteString("\n")
	screen.WriteString(resultsView)
	screen.WriteString("\n")
	screen.WriteString(footer)

	// Handle command palette overlay (same as main view)
	if m.CommandPalette {
		overlayWidth := min(60, m.WindowWidth-10)

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
		paletteContent.WriteString("Examples: 'query clear', 'query history', 'set queryTimeoutSeconds=60'\n")
		paletteContent.WriteString("Press Esc to close, Enter to execute\n\n")
		paletteContent.WriteString("> " + m.CommandInput)

		palette := paletteStyle.Render(paletteContent.String())

		// Overlay on screen
		lines := strings.Split(screen.String(), "\n")
		if len(lines) > 7 {
			startLine := len(lines)/2 - 2
			paletteLines := strings.Split(palette, "\n")
			for i, paletteLine := range paletteLines {
				if startLine+i < len(lines) && startLine+i >= 0 {
					lines[startLine+i] = paletteLine
				}
			}
			screen.Reset()
			screen.WriteString(strings.Join(lines, "\n"))
		}
	}

	return screen.String() + "\n"
}
