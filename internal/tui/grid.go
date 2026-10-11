package tui

import (
	"cmp"
	"slices"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// grid is a sortable, scrollable, clickable table, htop style: ctrl+alt+←→ pick the sort column, ctrl+alt+↑↓ (or alt+↑↓) order it,
// clicking a header sorts by it, clicking a row selects it. Rows keep their selection by id across
// refreshes.
type grid struct {
	cols   []gcol
	rows   []grow
	sel    int
	offset int
	sortBy int
	desc   bool
	id     string // zone id prefix
	shown  int    // rows that fit, from the last render
	x0     []int  // column start offsets, from the last render
	xcol   []int  // the column each x0 starts
	// simple are the columns the simple view shows; empty shows them all
	simple []int
	// want is a row id to select once a set brings it
	want string
	// xoff is how far the columns are scrolled sideways; total the width they need, vw what shows
	xoff, total, vw int
}

type gcol struct {
	title string
	width int // 0: share what is left
	right bool
	// cutLeft keeps a long cell's end (an image's tag), not its start
	cutLeft bool
}

type grow struct {
	id    string
	cells []string
	keys  []any // per column: float64 sorts numerically, anything else as text; nil uses the cell
	// pin keeps the row on top whatever the sort ("all instances")
	pin bool
}

func newGrid(id string, cols ...gcol) *grid {
	g := &grid{id: id, cols: cols, sortBy: -1}
	if s, ok := gridSorts[id]; ok && s.Col < len(cols) && cols[s.Col].title == s.Title {
		g.sortBy, g.desc = s.Col, s.Desc
	}
	return g
}

// gridSort is how the user last sorted a grid; gridSorts keeps it by grid id, so a grid built
// again (a screen reopened, a session resumed) sorts as it was left.
type gridSort struct {
	Col   int    `json:"col"`
	Title string `json:"title"`
	Desc  bool   `json:"desc"`
}

var gridSorts = map[string]gridSort{}

// sortDefault sorts on c unless the user sorted this grid before.
func (g *grid) sortDefault(c int, desc bool) {
	if _, ok := gridSorts[g.id]; !ok {
		g.sortBy, g.desc = c, desc
	}
}

func (g *grid) remember() {
	if g.sortBy >= 0 && g.sortBy < len(g.cols) {
		gridSorts[g.id] = gridSort{g.sortBy, g.cols[g.sortBy].title, g.desc}
	}
}

func col(title string, width int) gcol  { return gcol{title: title, width: width} }
func rcol(title string, width int) gcol { return gcol{title: title, width: width, right: true} }

// set replaces the rows, keeping the selected row (by id) selected, or selecting want once it shows.
func (g *grid) set(rows []grow) {
	cur := ""
	if g.sel < len(g.rows) {
		cur = g.rows[g.sel].id
	}
	if g.want != "" && slices.ContainsFunc(rows, func(r grow) bool { return r.id == g.want }) {
		cur, g.want = g.want, ""
	}
	g.rows = rows
	g.sortRows()
	g.sel = min(g.sel, max(0, len(g.rows)-1))
	for i, r := range g.rows {
		if r.id == cur && cur != "" {
			g.sel = i
		}
	}
}

func (g *grid) current() (grow, bool) {
	if g.sel < 0 || g.sel >= len(g.rows) {
		return grow{}, false
	}
	return g.rows[g.sel], true
}

func (g *grid) sortRows() {
	if g.sortBy < 0 || g.sortBy >= len(g.cols) {
		return
	}
	c := g.sortBy
	key := func(r grow) any {
		if c < len(r.keys) && r.keys[c] != nil {
			return r.keys[c]
		}
		if c < len(r.cells) {
			s := stripANSI(r.cells[c])
			if f, err := strconv.ParseFloat(strings.TrimRight(s, "%smµ/"), 64); err == nil {
				return f
			}
			return strings.ToLower(s)
		}
		return ""
	}
	sort.SliceStable(g.rows, func(i, j int) bool {
		if g.rows[i].pin != g.rows[j].pin {
			return g.rows[i].pin
		}
		c := compare(key(g.rows[i]), key(g.rows[j]))
		if c == 0 {
			c = strings.Compare(g.rows[i].id, g.rows[j].id)
		}
		if g.desc {
			return c > 0
		}
		return c < 0
	})
}

// compare orders numbers before text, numbers by value and text case-insensitively.
func compare(a, b any) int {
	af, aNum := a.(float64)
	bf, bNum := b.(float64)
	switch {
	case aNum && bNum:
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		}
		return 0
	case aNum:
		return -1
	case bNum:
		return 1
	}
	return strings.Compare(strings.ToLower(toString(a)), strings.ToLower(toString(b)))
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

func stripANSI(s string) string { return strings.TrimSpace(stripANSIKeepSpace(s)) }

func (g *grid) sortOn(c int) {
	if g.sortBy == c {
		g.desc = !g.desc
	} else {
		g.sortBy, g.desc = c, false
	}
	g.sortRows()
	g.remember()
}

// key handles movement and sorting keys; false when the key is not one of them.
func (g *grid) key(k tea.KeyPressMsg) bool {
	if listKeys(k, &g.sel, len(g.rows)) {
		return true
	}
	switch k.String() {
	case "left", "right":
		return g.scrollX(k.String() == "right")
	case "ctrl+alt+right", "ctrl+shift+right", ">":
		g.sortBy = (g.sortBy + 1) % len(g.cols)
		g.sortRows()
	case "ctrl+alt+left", "ctrl+shift+left", "<":
		g.sortBy = (max(g.sortBy, 0) - 1 + len(g.cols)) % len(g.cols)
		g.sortRows()
	// VTE (GNOME's terminal) keeps ctrl+shift+↑↓ and GNOME ctrl+alt+arrows: alt+↑↓ and < > always work
	case "ctrl+alt+up", "ctrl+alt+down", "ctrl+shift+up", "ctrl+shift+down", "alt+up", "alt+down":
		g.sortBy = max(g.sortBy, 0)
		switch k.String() {
		case "ctrl+alt+up", "ctrl+shift+up", "alt+up":
			g.desc = false
		case "ctrl+alt+down", "ctrl+shift+down", "alt+down":
			g.desc = true
		}
		g.sortRows()
	default:
		return false
	}
	g.remember()
	return true
}

// scrollX moves the columns sideways when they are wider than the grid; false (at either end, or
// when everything fits) lets the arrow go on to move focus.
func (g *grid) scrollX(right bool) bool {
	most := g.total - g.vw
	switch {
	case most <= 0:
		return false
	case right && g.xoff < most:
		g.xoff = min(most, g.xoff+8)
	case !right && g.xoff > 0:
		g.xoff = max(0, g.xoff-8)
	default:
		return false
	}
	return true
}

// click handles a hit on this grid's zones: true when it selected a row (activate on double).
func (g *grid) click(h hit) (selected bool) {
	switch h.id {
	case g.id + ":head":
		for i := len(g.x0) - 1; i >= 0; i-- {
			if h.x >= g.x0[i] {
				g.sortOn(g.xcol[i])
				return false
			}
		}
	case g.id + ":rows":
		if i := g.offset + h.y; i < len(g.rows) {
			g.sel = i
			return true
		}
	}
	return false
}

// shows are the columns drawn: the simple ones in the simple view, else all.
func (g *grid) shows(m *model) []int {
	if m != nil && m.simple && len(g.simple) > 0 {
		return g.simple
	}
	out := make([]int, len(g.cols))
	for i := range out {
		out[i] = i
	}
	return out
}

// widths of every column for width w; columns not in show get 0 and take no room.
func (g *grid) widths(w int, show []int) []int {
	ws := make([]int, len(g.cols))
	fixed, flex := len(show)-1, 0
	for _, i := range show {
		c := g.cols[i]
		ws[i] = cmp.Or(gridWidths[g.id+"\x00"+c.title], c.width)
		fixed += ws[i]
		if ws[i] == 0 {
			flex++
		}
	}
	if flex > 0 {
		each := max(6, (w-fixed)/flex)
		nat, sum := map[int]int{}, 0
		for _, i := range show {
			if ws[i] == 0 {
				nat[i] = g.natural(i)
				sum += nat[i]
			}
		}
		for _, i := range show {
			if ws[i] != 0 {
				continue
			}
			ws[i] = each
			if sum > w-fixed && flex > 1 {
				ws[i] = nat[i]
			}
		}
	}
	return ws
}

// natural is the width column c needs for its title and its first rows' cells, at most 40.
func (g *grid) natural(c int) int {
	n := lipgloss.Width(g.cols[c].title) + 1
	for i, r := range g.rows {
		if i == 200 {
			break
		}
		if c < len(r.cells) {
			n = max(n, lipgloss.Width(printable(r.cells[c])))
		}
	}
	return max(6, min(n, 40))
}

// view renders the grid in w×h cells at (x, y) of the tab body and registers its click zones.
func (g *grid) view(m *model, x, y, w, h int, focused bool) string {
	show := g.shows(m)
	ws := g.widths(w, show)
	g.total, g.vw = len(show)-1, w
	for _, i := range show {
		g.total += ws[i]
	}
	g.xoff = max(0, min(g.xoff, g.total-w))
	g.x0, g.xcol = g.x0[:0], g.xcol[:0]
	var head []string
	off := 0
	for _, i := range show {
		c := g.cols[i]
		g.x0, g.xcol = append(g.x0, off-g.xoff), append(g.xcol, i)
		t := c.title
		if i == g.sortBy {
			if g.desc {
				t += "▼"
			} else {
				t += "▲"
			}
		}
		cell := padRight(t, ws[i])
		if c.right {
			cell = padLeft(t, ws[i])
		}
		if i == g.sortBy {
			cell = sAccent.Render(cell)
		} else {
			cell = sDim.Render(cell)
		}
		head = append(head, cell)
		off += ws[i] + 1
	}
	cellX := make([]int, len(g.cols))
	for k, c := range g.xcol {
		cellX[c] = g.x0[k]
	}
	if m != nil {
		if m.grids == nil {
			m.grids = map[string]*grid{}
		}
		m.grids[g.id] = g
		if m.hovering(x, y, w, 1) {
			// drawn │ gaps show where a column's edge can be dragged
			head = []string{strings.Join(head, sDim.Render("│"))}
		}
	}
	g.shown = max(0, h-1)
	g.offset = scroll(g.sel, g.offset, g.shown, len(g.rows))
	var b strings.Builder
	clip := func(l string) string {
		if g.total <= w {
			return l
		}
		return ansi.Cut(l, g.xoff, g.xoff+w)
	}
	hl := clip(strings.Join(head, " "))
	if g.xoff > 0 {
		hl = sAmber.Render("‹") + ansi.Cut(hl, 1, w)
	}
	if g.xoff < g.total-w {
		hl = ansi.Cut(hl, 0, w-1) + sAmber.Render("›")
	}
	b.WriteString(hl)
	for i := g.offset; i < len(g.rows) && i-g.offset < g.shown; i++ {
		r := g.rows[i]
		cells := make([]string, 0, len(show))
		for _, j := range show {
			c := g.cols[j]
			v := ""
			if j < len(r.cells) {
				v = printable(r.cells[j])
			}
			if m != nil {
				m.tipFor(v, x+cellX[j], y+1+i-g.offset+m.originY, ws[j])
			}
			if c.cutLeft && lipgloss.Width(v) > ws[j] {
				v = ansi.TruncateLeft(v, lipgloss.Width(v)-ws[j]+1, "…")
			}
			if c.right {
				cells = append(cells, padLeft(v, ws[j]))
			} else {
				cells = append(cells, padRight(v, ws[j]))
			}
		}
		line := clip(strings.Join(cells, " "))
		if i != g.sel && m.hovering(x, y+1+i-g.offset, w, 1) {
			line = highlight(sHover, line, w)
		} else if i == g.sel && focused {
			line = highlight(sSelected, line, w)
		} else if i == g.sel {
			line = highlight(sUnderline, line, w)
		}
		b.WriteString("\n" + line)
	}
	if len(g.rows) == 0 {
		b.WriteString("\n" + sDim.Render("nothing yet"))
	}
	m.zone(g.id+":head", x, y, w, 1)
	m.zone(g.id+":rows", x, y+1, w, g.shown)
	return b.String()
}

// highlight lays st (a background, bold, underline) over a styled line padded to w and keeps the
// cells' own colours: lipgloss would end st at the first reset inside the line, or, for underline,
// split the escape codes into visible text, so st's codes are re-opened after every reset instead.
func highlight(st lipgloss.Style, line string, w int) string {
	open, _, _ := strings.Cut(st.Render("\x00"), "\x00")
	line = padRight(line, w)
	if open == "" {
		return line
	}
	line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+open)
	line = strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+open)
	return open + line + "\x1b[0m"
}

func stripANSIKeepSpace(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'):
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func padLeft(s string, w int) string {
	s = ltr(s)
	if d := w - lipgloss.Width(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return closeLTR(s, truncate(s, w))
}

// printable keeps a cell one line of known width: a tab or newline from SQL text or kubectl output
// moves the terminal's cursor where lipgloss does not count it, leaving stale text of the last frame.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case r < 0x20 && r != '\x1b', r == 0x7f:
			return -1
		}
		return r
	}, s)
}
