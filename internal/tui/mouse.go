package tui

import tea "charm.land/bubbletea/v2"

type mouseAction int

const (
	mousePress mouseAction = iota
	mouseRelease
	mouseMotion
)

// mouseEvent is one mouse message of any kind: the screen handles presses, releases, drags and the
// wheel in one place.
type mouseEvent struct {
	tea.Mouse
	Action           mouseAction
	Shift, Alt, Ctrl bool
}

func mouseOf(msg tea.MouseMsg) mouseEvent {
	e := mouseEvent{Mouse: msg.Mouse()}
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		e.Action = mouseRelease
	case tea.MouseMotionMsg:
		e.Action = mouseMotion
	}
	e.Shift, e.Alt, e.Ctrl = e.Mod.Contains(tea.ModShift), e.Mod.Contains(tea.ModAlt), e.Mod.Contains(tea.ModCtrl)
	return e
}
