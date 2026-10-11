package tui

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/goxang/jig/api"
	"github.com/goxang/jig/contracts"
	"github.com/goxang/jig/engine"
	"github.com/goxang/jig/host"
	"github.com/goxang/jig/kernel/frames"
	"github.com/goxang/jig/kernel/view"
	"github.com/goxang/jig/protocol"
)

// viewTab is a screen drawn from a view spec: a widget tree whose bound widgets the host fetches,
// only while they show, sharing one request per distinct source and args.
type viewTab struct {
	title string
	v     *view.View
	// dir is where the view's file: and widgets' file: props resolve
	dir string
	// remote is <plugin>.<view> for a view its plugin renders (render)
	remote string

	root  *wnode
	eng   *engine.Local
	fetch *view.Fetcher
	ctx   context.Context
	stop  context.CancelFunc
	focus *wnode
	// order is every focusable widget in the last frame, in draw order
	order []*wnode
	err   string
}

// wnode is a widget of the tree with its live state.
type wnode struct {
	n     protocol.Node
	kids  []*wnode
	frame *protocol.Frame
	err   error
	// loading is a fetch in flight; streaming a stream feeding the widget
	loading, streaming bool
	// tab is a tabs widget's shown child
	tab        int
	x, y, w, h int
	id         int
	st         any
}

type viewDataMsg struct {
	t   *viewTab
	n   *wnode
	f   *protocol.Frame
	err error
}

type viewStreamMsg struct {
	t    *viewTab
	n    *wnode
	f    *protocol.Frame
	done bool
	err  error
}

func newViewTab(title string, v *view.View, dir string) *viewTab {
	return &viewTab{title: title, v: v, dir: dir}
}

func (t *viewTab) name() string { return t.title }

func (t *viewTab) typing() bool {
	if t.focus == nil {
		return false
	}
	switch st := t.focus.st.(type) {
	case *terminal:
		return st.input
	}
	return false
}

func (t *viewTab) interval() time.Duration { return time.Second }

func (t *viewTab) build() {
	t.err = ""
	if t.remote != "" && t.v.Root.Widget == "" {
		return
	}
	if t.v.File != "" && t.v.Root.Widget == "" {
		raw, err := os.ReadFile(filepath.Join(t.dir, t.v.File))
		if err == nil {
			err = t.v.Load(raw)
		}
		if err != nil {
			t.err = fmt.Sprintf("%s: %v", t.v.File, err)
			return
		}
	}
	id := 0
	var mk func(n protocol.Node) *wnode
	mk = func(n protocol.Node) *wnode {
		id++
		wn := &wnode{n: n, id: id}
		for _, c := range n.Children {
			wn.kids = append(wn.kids, mk(c))
		}
		return wn
	}
	t.root = mk(t.v.Root)
}

func (t *viewTab) open(m *model) tea.Cmd {
	if t.root == nil {
		t.build()
	}
	if t.root == nil && t.remote != "" && t.err == "" {
		e, name := m.eng(), t.remote
		return func() tea.Msg {
			n, err := e.Host.View(context.Background(), name, nil)
			return viewRootMsg{t: t, n: n, err: err}
		}
	}
	return t.refresh(m)
}

type viewRootMsg struct {
	t   *viewTab
	n   *protocol.Node
	err error
}

func (t *viewTab) hide(m *model) {
	if t.stop != nil {
		t.stop()
		t.stop = nil
	}
	t.walkVisible(func(wn *wnode) {
		wn.loading, wn.streaming = false, false
		if term, ok := wn.st.(*terminal); ok && !term.keep {
			term.close()
			wn.st = nil
		}
	})
}

// walkVisible visits the widgets that show: only the open child of each tabs widget.
func (t *viewTab) walkVisible(fn func(*wnode)) {
	var walk func(*wnode)
	walk = func(wn *wnode) {
		if wn == nil {
			return
		}
		fn(wn)
		if view.Kind(wn.n) == "tabs" {
			if wn.tab < len(wn.kids) {
				walk(wn.kids[wn.tab])
			}
			return
		}
		for _, k := range wn.kids {
			walk(k)
		}
	}
	walk(t.root)
}

func (t *viewTab) refresh(m *model) tea.Cmd {
	if t.root == nil {
		return nil
	}
	if e := m.eng(); t.eng != e {
		t.eng, t.fetch = e, &view.Fetcher{Query: e.Query}
	}
	if t.ctx == nil || t.ctx.Err() != nil {
		t.ctx, t.stop = context.WithCancel(m.ctx)
	}
	var cmds []tea.Cmd
	t.walkVisible(func(wn *wnode) {
		if wn.n.Bind == nil || wn.n.Bind.Source == "" || wn.loading || wn.streaming {
			return
		}
		if streams(wn.n) {
			cmds = append(cmds, t.stream(wn))
			return
		}
		if wn.frame != nil && t.fetch.Fresh(*wn.n.Bind) {
			return
		}
		cmds = append(cmds, t.load(wn))
	})
	return batch(cmds...)
}

func streams(n protocol.Node) bool {
	s, _ := n.Props["stream"].(bool)
	return s
}

func (t *viewTab) load(wn *wnode) tea.Cmd {
	wn.loading = true
	b, fetch, ctx := *wn.n.Bind, t.fetch, t.ctx
	ops := wn.n.Props["transform"]
	return func() tea.Msg {
		f, err := fetch.Get(ctx, b)
		if err == nil {
			f, err = view.Apply(f, ops, func(o protocol.Binding) (*protocol.Frame, error) { return fetch.Get(ctx, o) })
		}
		return viewDataMsg{t: t, n: wn, f: f, err: err}
	}
}

// stream feeds a widget from a streaming source: each batch's rows append, up to keep: (5000).
func (t *viewTab) stream(wn *wnode) tea.Cmd {
	wn.streaming = true
	b, e, ctx := *wn.n.Bind, t.eng, t.ctx
	return func() tea.Msg {
		ch, err := e.Stream(ctx, b.Source, b.Args)
		if err != nil {
			return viewStreamMsg{t: t, n: wn, done: true, err: err}
		}
		go func() {
			for batch := range ch {
				msg := viewStreamMsg{t: t, n: wn, f: batch.Frame, done: batch.Done}
				if len(batch.Lines) > 0 {
					rows := make([]map[string]any, len(batch.Lines))
					for i, l := range batch.Lines {
						rows[i] = map[string]any{"line": l}
					}
					msg.f = frames.FromRows([]protocol.Field{{Name: "line", Type: protocol.TypeString}}, rows)
				}
				if batch.Error != nil {
					msg.err = batch.Error
				}
				program.Send(msg)
			}
			program.Send(viewStreamMsg{t: t, n: wn, done: true})
		}()
		return nil
	}
}

func (t *viewTab) update(m *model, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case viewDataMsg:
		if msg.t != t {
			return nil
		}
		msg.n.loading = false
		if msg.err != nil && msg.t.ctx.Err() != nil {
			return nil
		}
		msg.n.err = msg.err
		if msg.f != nil || msg.err == nil {
			msg.n.frame = msg.f
		}
		return nil
	case viewStreamMsg:
		if msg.t != t {
			return nil
		}
		wn := msg.n
		if msg.err != nil {
			wn.err = msg.err
		}
		if msg.f != nil {
			wn.frame = appendFrame(wn.frame, msg.f, keepRows(wn.n))
		}
		if msg.done {
			wn.streaming = false
		}
		return nil
	case viewRootMsg:
		if msg.t != t {
			return nil
		}
		if msg.err != nil {
			t.err = t.remote + ": " + msg.err.Error()
			return nil
		}
		t.v.Root = *msg.n
		t.build()
		return t.refresh(m)
	case canvasMsg:
		msg.cv.busy, msg.cv.lines, msg.cv.err = false, msg.lines, msg.err
	case tea.KeyPressMsg:
		return t.key(m, msg)
	case tea.PasteMsg:
		if term, ok := t.focusTerm(); ok && term.input {
			term.paste(msg.Content)
		}
	}
	return nil
}

func keepRows(n protocol.Node) int {
	if k, err := strconv.Atoi(view.Prop(n, "keep")); err == nil && k > 0 {
		return k
	}
	return 5000
}

// appendFrame adds b's rows under a's, keeping the last keep rows.
func appendFrame(a, b *protocol.Frame, keep int) *protocol.Frame {
	if a == nil || len(a.Fields) != len(b.Fields) {
		a = &protocol.Frame{Fields: b.Fields, Columns: make([][]any, len(b.Fields))}
	}
	for i := range a.Columns {
		if i < len(b.Columns) {
			a.Columns[i] = append(a.Columns[i], b.Columns[i]...)
		}
		if over := len(a.Columns[i]) - keep; over > 0 {
			a.Columns[i] = a.Columns[i][over:]
		}
	}
	return a
}

func (t *viewTab) focusTerm() (*terminal, bool) {
	if t.focus == nil {
		return nil, false
	}
	term, ok := t.focus.st.(*terminal)
	return term, ok
}

func (t *viewTab) key(m *model, k tea.KeyPressMsg) tea.Cmd {
	if term, ok := t.focusTerm(); ok && term.input {
		if k.String() == "ctrl+]" {
			term.input = false
			return nil
		}
		term.key(k)
		return nil
	}
	switch k.String() {
	case "left", "right":
		t.moveFocus(map[string]int{"left": -1, "right": 1}[k.String()])
		return nil
	case "ctrl+left", "ctrl+right", "shift+left", "shift+right":
		if tb := t.tabsOf(t.focus); tb != nil && len(tb.kids) > 0 {
			d := 1
			if strings.HasSuffix(k.String(), "left") {
				d = -1
			}
			t.hide(m)
			tb.tab = (tb.tab + d + len(tb.kids)) % len(tb.kids)
			t.focus = nil
			return t.refresh(m)
		}
		return nil
	case "r":
		if t.fetch != nil {
			t.fetch.Forget()
		}
		t.walkVisible(func(wn *wnode) { wn.loading = false })
		return t.refresh(m)
	}
	if t.focus == nil {
		return nil
	}
	if cmd, ok := t.rowAction(m, t.focus, k.String()); ok {
		return cmd
	}
	return t.widgetKey(m, t.focus, k)
}

// tabsOf is the tabs widget that holds wn, else the first one showing.
func (t *viewTab) tabsOf(wn *wnode) *wnode {
	var found, first *wnode
	var walk func(n *wnode, tabs *wnode)
	walk = func(n *wnode, tabs *wnode) {
		if n == nil {
			return
		}
		if view.Kind(n.n) == "tabs" {
			tabs = n
			if first == nil {
				first = n
			}
		}
		if n == wn && tabs != nil {
			found = tabs
		}
		for _, k := range n.kids {
			walk(k, tabs)
		}
	}
	walk(t.root, nil)
	if found != nil {
		return found
	}
	return first
}

func (t *viewTab) moveFocus(d int) {
	if len(t.order) == 0 {
		return
	}
	i := -1
	for j, wn := range t.order {
		if wn == t.focus {
			i = j
		}
	}
	t.focus = t.order[(i+d+len(t.order))%len(t.order)]
}

// rowAction runs a table action bound to key: actions: [{key: R, action: x.retry, args: {id: "${row.id}"}}].
func (t *viewTab) rowAction(m *model, wn *wnode, key string) (tea.Cmd, bool) {
	acts, _ := wn.n.Props["actions"].([]any)
	for _, a := range acts {
		am, _ := a.(map[string]any)
		if fmt.Sprint(am["key"]) != key {
			continue
		}
		row := t.currentRow(wn)
		if c, ok := am["command"].(string); ok {
			label, line := fmt.Sprint(am["name"]), fmt.Sprint(expandRow(c, row))
			if danger, _ := am["danger"].(bool); danger {
				m.confirm = &confirm{text: label + ": jig " + line + "?", run: func() tea.Msg { return commandMsg(line) }}
				return nil, true
			}
			return m.commandAsking(label, line), true
		}
		name := fmt.Sprint(am["action"])
		args := protocol.Args{}
		if raw, ok := am["args"].(map[string]any); ok {
			for k, v := range raw {
				args[k] = expandRow(v, row)
			}
		}
		label := fmt.Sprint(am["name"])
		if label == "<nil>" {
			label = name
		}
		return t.run(m, label, name, args), true
	}
	return nil, false
}

// run runs an action through the Engine, so policy, confirmation and audit apply.
func (t *viewTab) run(m *model, label, action string, args protocol.Args) tea.Cmd {
	e := m.eng()
	run := func(ctx context.Context) error {
		_, res, err := e.Run(ctx, action, args, api.RunOpts{})
		if err == nil && res != nil && res.Message != "" {
			program.Send(statusMsg{text: res.Message})
		}
		return err
	}
	if a, ok := e.Action(action); ok && a.Effect == protocol.EffectRead {
		return m.do(label, run)
	}
	refresh := func(ctx context.Context) error {
		err := run(ctx)
		if t.fetch != nil {
			t.fetch.Forget()
		}
		return err
	}
	return m.act(label, actionDangerous(e, action), refresh)
}

func actionDangerous(e *engine.Local, name string) bool {
	a, ok := e.Action(name)
	return ok && a.Effect == protocol.EffectDanger
}

// expandRow fills ${row.col} from the selected row.
func expandRow(v any, row map[string]any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if strings.HasPrefix(s, "${row.") && strings.HasSuffix(s, "}") && strings.Count(s, "${") == 1 {
		return row[s[6:len(s)-1]]
	}
	for k, x := range row {
		s = strings.ReplaceAll(s, "${row."+k+"}", fmt.Sprint(x))
	}
	return s
}

func (t *viewTab) currentRow(wn *wnode) map[string]any {
	if wn.frame == nil {
		return nil
	}
	if g, ok := wn.st.(*grid); ok {
		if r, ok := g.current(); ok {
			if i, err := strconv.Atoi(r.id); err == nil && i < frames.Len(wn.frame) {
				return frames.Row(wn.frame, i)
			}
		}
	}
	if frames.Len(wn.frame) > 0 {
		return frames.Row(wn.frame, 0)
	}
	return nil
}

func (t *viewTab) hints() [][2]string {
	h := [][2]string{{"←→", "focus"}, {"r", "refresh"}}
	if t.tabsOf(nil) != nil {
		h = append(h, [2]string{"ctrl+←→", "tabs"})
	}
	if t.focus != nil {
		switch view.Kind(t.focus.n) {
		case "table":
			h = append(h, [2]string{"enter", "open"}, [2]string{"y Y", "copy"})
		case "terminal":
			h = append(h, [2]string{"enter", "type into it"}, [2]string{"R", "restart"})
		case "form":
			h = append(h, [2]string{"enter", "edit"}, [2]string{"ctrl+s", "submit"})
		}
		acts, _ := t.focus.n.Props["actions"].([]any)
		for _, a := range acts {
			am, _ := a.(map[string]any)
			h = append(h, [2]string{fmt.Sprint(am["key"]), fmt.Sprint(firstNonNil(am["name"], am["action"]))})
		}
	}
	return h
}

func firstNonNil(v ...any) any {
	for _, x := range v {
		if x != nil {
			return x
		}
	}
	return nil
}

func (t *viewTab) view(m *model, w, h int) string {
	if t.err != "" {
		return sRed.Render(t.err)
	}
	if t.root == nil {
		return ""
	}
	t.order = t.order[:0]
	out := t.draw(m, t.root, 0, 0, w, h)
	if t.focus == nil && len(t.order) > 0 {
		t.focus = t.order[0]
	}
	return out
}

// draw lays wn out in exactly w×h cells at x, y of the body.
func (t *viewTab) draw(m *model, wn *wnode, x, y, w, h int) string {
	wn.x, wn.y, wn.w, wn.h = x, y, w, h
	if w <= 0 || h <= 0 {
		return ""
	}
	switch view.Kind(wn.n) {
	case "grid", "stack":
		return t.stackDraw(m, wn.kids, x, y, w, h, true)
	case "row":
		return t.stackDraw(m, wn.kids, x, y, w, h, false)
	case "split":
		return t.stackDraw(m, wn.kids, x, y, w, h, view.Prop(wn.n, "dir") == "v" || view.Prop(wn.n, "dir") == "vertical")
	case "tabs":
		return t.tabsDraw(m, wn, x, y, w, h)
	case "box":
		if len(wn.kids) == 0 {
			return panel(view.Prop(wn.n, "title"), "", w, h, false)
		}
		return panel(view.Prop(wn.n, "title"), t.draw(m, wn.kids[0], x+1, y+1, w-2, h-2), w, h, false)
	case "scroll":
		return t.scrollDraw(m, wn, x, y, w, h)
	}
	t.order = append(t.order, wn)
	focused := wn == t.focus
	m.zone(fmt.Sprintf("view:%d", wn.id), x, y, w, h)
	if b, ok := wn.n.Props["border"].(bool); ok && !b {
		return clip(t.leaf(m, wn, x, y, w, h, focused), w, h)
	}
	title := view.Prop(wn.n, "title")
	if term, ok := wn.st.(*terminal); ok && term.input {
		title += sDim.Render(" · ctrl+] leaves")
	}
	if wn.loading && wn.frame == nil {
		title += " " + sDim.Render("…")
	}
	return panel(title, t.leaf(m, wn, x+1, y+1, w-2, h-2, focused), w, h, focused)
}

// stackDraw splits w×h among kids: by height: or span: when given, else evenly.
func (t *viewTab) stackDraw(m *model, kids []*wnode, x, y, w, h int, vertical bool) string {
	if len(kids) == 0 {
		return ""
	}
	total := h
	if !vertical {
		total = w
	}
	sizes := shares(kids, total, vertical)
	var parts []string
	off := 0
	for i, k := range kids {
		if vertical {
			parts = append(parts, t.draw(m, k, x, y+off, w, sizes[i]))
		} else {
			parts = append(parts, t.draw(m, k, x+off, y, sizes[i], h))
		}
		off += sizes[i]
	}
	if vertical {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// shares gives each kid its fixed size (height: rows, or a percentage), then splits what is left
// by span: weights.
func shares(kids []*wnode, total int, vertical bool) []int {
	out := make([]int, len(kids))
	key := "width"
	if vertical {
		key = "height"
	}
	left, weights := total, 0
	for i, k := range kids {
		if n, ok := size(k, key, total); ok {
			out[i] = min(n, left)
			left -= out[i]
			continue
		}
		weights += span(k)
	}
	for i, k := range kids {
		if out[i] == 0 && weights > 0 {
			out[i] = left * span(k) / weights
		}
	}
	// the last flexible kid takes what rounding left
	used := 0
	for _, s := range out {
		used += s
	}
	for i := len(kids) - 1; i >= 0 && used < total; i-- {
		if _, fixed := size(kids[i], key, total); !fixed {
			out[i] += total - used
			break
		}
	}
	return out
}

func size(wn *wnode, key string, total int) (int, bool) {
	s := view.Prop(wn.n, key)
	if wn.n.Widget == "row" && len(wn.kids) > 0 && key == "height" {
		best := 0
		for _, k := range wn.kids {
			if n, ok := size(k, key, total); ok {
				best = max(best, n)
			}
		}
		return best, best > 0
	}
	if s == "" {
		return 0, false
	}
	if p, ok := strings.CutSuffix(s, "%"); ok {
		n, err := strconv.Atoi(p)
		return total * n / 100, err == nil
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n > 0
}

func span(wn *wnode) int {
	if n, err := strconv.Atoi(view.Prop(wn.n, "span")); err == nil && n > 0 {
		return n
	}
	return 1
}

func (t *viewTab) tabsDraw(m *model, wn *wnode, x, y, w, h int) string {
	var strip []string
	cx := x
	for i, k := range wn.kids {
		title := view.Prop(k.n, "title")
		if title == "" {
			title = strconv.Itoa(i + 1)
		}
		st := sSubOff
		if i == wn.tab {
			st = sSubOn
		}
		s := st.Render(title)
		m.zone(fmt.Sprintf("viewtab:%d:%d", wn.id, i), cx, y, lipgloss.Width(s), 1)
		cx += lipgloss.Width(s) + 1
		strip = append(strip, s)
	}
	head := padRight(strings.Join(strip, sSubSep), w)
	if wn.tab >= len(wn.kids) {
		return head
	}
	return head + "\n" + t.draw(m, wn.kids[wn.tab], x, y+1, w, h-1)
}

// scrollDraw draws the child taller (height:, else twice the box) and shows a window of it.
func (t *viewTab) scrollDraw(m *model, wn *wnode, x, y, w, h int) string {
	if len(wn.kids) == 0 {
		return ""
	}
	full, ok := size(wn, "height", h)
	if !ok || full < h {
		full = 2 * h
	}
	off, _ := wn.st.(int)
	off = max(0, min(off, full-h))
	wn.st = off
	t.order = append(t.order, wn)
	lines := strings.Split(t.draw(m, wn.kids[0], x, y-off, w, full), "\n")
	end := min(off+h, len(lines))
	return strings.Join(lines[min(off, end):end], "\n")
}

func (t *viewTab) click(m *model, h hit) tea.Cmd {
	var id, sub int
	if n, _ := fmt.Sscanf(h.id, "viewtab:%d:%d", &id, &sub); n == 2 {
		var cmd tea.Cmd
		t.walkVisible(func(wn *wnode) {
			if wn.id == id && sub < len(wn.kids) && wn.tab != sub {
				t.hide(m)
				wn.tab, t.focus = sub, nil
				cmd = t.refresh(m)
			}
		})
		return cmd
	}
	if n, _ := fmt.Sscanf(h.id, "view:%d", &id); n == 1 {
		for _, wn := range t.order {
			if wn.id == id {
				t.focus = wn
				return t.widgetClick(m, wn, h)
			}
		}
	}
	if t.focus != nil {
		if g, ok := t.focus.st.(*grid); ok && strings.HasPrefix(h.id, g.id) {
			g.click(h)
		}
	}
	return nil
}

func (t *viewTab) wheel(m *model, z hit, up bool) (tea.Cmd, bool) {
	k := tea.KeyPressMsg{Code: tea.KeyDown}
	if up {
		k = tea.KeyPressMsg{Code: tea.KeyUp}
	}
	if t.focus == nil {
		return nil, false
	}
	if term, ok := t.focus.st.(*terminal); ok {
		term.scrollBy(map[bool]int{true: 3, false: -3}[up])
		return nil, true
	}
	return t.widgetKey(m, t.focus, k), true
}

// atRoot: esc has nothing left to close here.
func (t *viewTab) atRoot() bool { return true }

// addViews puts the project's views and the plugins' views with slot tab into all, keeping the
// ones already there (and their state), and returns them in order.
func addViews(a *engine.App, all map[string]tab) []tab {
	var out []tab
	var home []protocol.Node
	servedBy := map[*viewTab]string{}
	add := func(key, title string, v *view.View, dir, remote string) {
		if v.Slot == "home" {
			tile := v.Root
			tile.Props = maps.Clone(tile.Props)
			if tile.Props == nil {
				tile.Props = protocol.Args{}
			}
			tile.Props["title"] = cmp.Or(view.Prop(tile, "title"), v.Title, title)
			home = append(home, tile)
			return
		}
		if v.Slot != "tab" {
			return
		}
		t, ok := all[key].(*viewTab)
		if !ok {
			t = newViewTab(cmp.Or(v.Title, title), v, dir)
			t.remote = remote
			all[key] = t
		}
		if plugin, _, ok := strings.Cut(strings.TrimPrefix(key, "view:"), "/"); ok {
			t.title = cmp.Or(v.Title, title)
			servedBy[t] = plugin
		}
		out = append(out, t)
	}
	for _, n := range a.Spec.ViewOrder {
		if v := a.Spec.Views[n]; v != nil {
			add("view:"+n, n, v, a.Spec.Dir, "")
		}
	}
	ms, _ := host.Discover(a.Spec.Dir, a.Spec.Plugins)
	for _, m := range ms {
		if !m.Relevant(a.Spec.Dir, a.Spec.Plugins) {
			continue
		}
		names := make([]string, 0, len(m.Views))
		for n := range m.Views {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			node := m.Views[n]
			var v view.View
			if err := a.Spec.Expanded(&node).Decode(&v); err != nil {
				v = view.View{Slot: "tab", Root: protocol.Node{Widget: "text", Props: protocol.Args{"text": err.Error()}}}
			}
			remote := ""
			if v.Root.Widget == "" && v.File == "" {
				remote = m.Name + "." + n
			}
			add("view:"+m.Name+"."+n, n, &v, m.Dir, remote)
		}
		// a plugin with its own views has laid out what the contract views would show
		for _, id := range m.Implements {
			if len(names) > 0 {
				break
			}
			if c := contracts.Get(id); c != nil && c.View != nil {
				var v view.View
				if err := yaml.Unmarshal(c.View, &v); err != nil {
					panic(err)
				}
				served(&v.Root, m.Name)
				add("view:"+m.Name+"/"+c.ID, v.Title, &v, m.Dir, "")
			}
		}
	}
	// two plugins serving one contract (docs from files and from Confluence) are named after the plugins
	titles := map[string]int{}
	for t := range servedBy {
		titles[t.title]++
	}
	for t, plugin := range servedBy {
		if titles[t.title] > 1 {
			t.title = strings.ToUpper(plugin[:1]) + strings.ReplaceAll(plugin[1:], "-", " ")
		}
	}
	if len(ms) > 0 || len(a.Spec.Plugins) > 0 {
		v := view.View{Slot: "tab"}
		if err := v.Load(pluginsView); err != nil {
			panic(err)
		}
		add("view:Plugins", "Plugins", &v, a.Spec.Dir, "")
	}
	if len(home) > 0 {
		root := protocol.Node{Widget: "grid"}
		for i := 0; i < len(home); i += 2 {
			root.Children = append(root.Children, protocol.Node{Widget: "row", Children: home[i:min(i+2, len(home))]})
		}
		add("view:Home", "Home", &view.View{Title: "Home", Slot: "tab", Root: root}, a.Spec.Dir, "")
		out = append([]tab{out[len(out)-1]}, out[:len(out)-1]...)
	}
	return out
}

// served points a contract's standard view at the plugin serving it: scm.mrs becomes <plugin>.scm.mrs.
func served(root *protocol.Node, plugin string) {
	view.Walk(root, func(n *protocol.Node) {
		if n.Bind != nil && n.Bind.Source != "" {
			n.Bind.Source = plugin + "." + n.Bind.Source
		}
		acts, _ := n.Props["actions"].([]any)
		for _, a := range acts {
			if am, ok := a.(map[string]any); ok && am["action"] != nil {
				am["action"] = plugin + "." + fmt.Sprint(am["action"])
			}
		}
	})
}

// pluginsView lists the plugins in use; its keys run `jig plugin ...`.
var pluginsView = []byte(`
title: Plugins
widget: table
source: plugins
every: 5s
actions:
  - {key: i, name: info, command: "plugin info ${row.name}"}
  - {key: l, name: logs, command: "plugin logs ${row.name}"}
  - {key: s, name: search the registry, command: "plugin search ${ask}"}
  - {key: N, name: add, command: "plugin add ${ask}"}
  - {key: S, name: turn on, command: "plugin on ${row.name}"}
  - {key: X, name: turn off, command: "plugin off ${row.name}"}
  - {key: U, name: upgrade, command: "plugin up ${row.name}"}
  - {key: D, name: remove, command: "plugin rm ${row.name}", danger: true}
`)
