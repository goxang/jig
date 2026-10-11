package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// hoverDelay is how long the pointer rests before a cut-off cell shows its whole text.
const hoverDelay = 500 * time.Millisecond

type hoverMsg struct{ at time.Time }

// tooltip is the whole text of the cell under a resting pointer, drawn over the frame there.
type tooltip struct {
	x, y int
	text string
}

// colDrag is a table column's right edge being dragged in the grid id.
type colDrag struct {
	g     *grid
	col   int
	start int
}

// gridWidths are the column widths dragged by hand, by grid id and column title, for this session.
var gridWidths = map[string]int{}

// moved restarts the rest timer: the tooltip goes, and comes back if the pointer stays put.
func (m *model) moved() tea.Cmd {
	m.movedAt, m.still, m.tip = time.Now(), false, nil
	at := m.movedAt
	return tea.Tick(hoverDelay, func(time.Time) tea.Msg { return hoverMsg{at: at} })
}

// tipFor offers the full text of a cell drawn cut off at screen row y, columns x..x+w.
func (m *model) tipFor(text string, x, y, w int) {
	if !m.still || m.hy != y || m.hx < x || m.hx >= x+w || lipgloss.Width(text) <= w {
		return
	}
	m.tip = &tooltip{x: x, y: y + 1, text: text}
}

// drawTip puts the tooltip under the cell, kept on screen, wrapped when wider than half of it.
func (m *model) drawTip(frame string) string {
	if m.tip == nil {
		return frame
	}
	w := min(lipgloss.Width(m.tip.text), max(20, m.w/2))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(0, 1).
		Render(strings.TrimRight(wrap(m.tip.text, w), "\n"))
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	x, y := min(m.tip.x, max(0, m.w-bw)), m.tip.y
	if y+bh > m.h {
		y = max(0, m.tip.y-1-bh)
	}
	return overlayAt(frame, box, x, y)
}

// startColDrag begins dragging when a press lands on the gap after a column in a grid's header.
func (m *model) startColDrag(e mouseEvent) bool {
	for _, z := range m.zones {
		id, ok := strings.CutSuffix(z.id, ":head")
		g := m.grids[id]
		if !ok || g == nil || e.Y != z.y || e.X < z.x || e.X >= z.x+z.w {
			continue
		}
		for i := 1; i < len(g.x0); i++ {
			if e.X-z.x == g.x0[i]-1 {
				c := g.xcol[i-1]
				m.colDrag = &colDrag{g: g, col: c, start: z.x + g.x0[i-1]}
				return true
			}
		}
	}
	return false
}

func (m *model) dragCol(e mouseEvent) {
	d := m.colDrag
	gridWidths[d.g.id+"\x00"+d.g.cols[d.col].title] = max(3, e.X-d.start)
	if e.Action == mouseRelease {
		m.colDrag = nil
	}
}
