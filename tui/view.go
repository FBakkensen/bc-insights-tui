package tui

import (
	"fmt"
)

// View renders the layout: top viewport and bottom textarea, centered.
func (m model) View() string {
	if m.quitting {
		return ""
	}
	top := m.vpStyle.Render(m.vp.View())
	bottom := m.ta.View()
	return m.containerStyle.Render(fmt.Sprintf("%s\n%s", top, bottom))
}
