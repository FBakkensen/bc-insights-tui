package tui

import (
	"fmt"
)

// View renders the layout: top viewport and bottom textarea, centered.
func (m model) View() string {
	if m.quitting {
		return ""
	}
	var top string
	switch m.mode {
	case modeListSubscriptions, modeListInsightsResources:
		top = m.vpStyle.Render(m.list.View())
	case modeTableResults:
		top = m.vpStyle.Render(m.tbl.View())
	case modeKQLEditor:
		// Show scrollback in top area while editing
		top = m.vpStyle.Render(m.vp.View())
	default:
		top = m.vpStyle.Render(m.vp.View())
	}
	bottom := m.ta.View()
	return m.containerStyle.Render(fmt.Sprintf("%s\n%s", top, bottom))
}
