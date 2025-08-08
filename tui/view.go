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
	if m.mode == modeListSubscriptions {
		top = m.vpStyle.Render(m.list.View())
	} else {
		top = m.vpStyle.Render(m.vp.View())
	}
	bottom := m.ta.View()
	return m.containerStyle.Render(fmt.Sprintf("%s\n%s", top, bottom))
}
