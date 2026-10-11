package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// filesTab browses the project's directory: folders, a preview of the selected file, and the
// editor or another app to open it.
type filesTab struct {
	root, dir string
	list      *grid
	filter    string
	err       string
	// preview is the selected file's rendered text, for previewFor
	preview, previewFor string
	scroll              int
	// query is a search over the whole project (/ names and content, f names), hits its results
	query, mode string
	hits        []fileHit
	results     *grid
	found       bool
	idx         fileIndex
	seq         int
}

type foundMsg struct {
	seq  int
	hits []fileHit
}

type filesMsg struct {
	dir     string
	entries []os.DirEntry
	err     error
}

func (t *filesTab) name() string { return "Files" }
func (t *filesTab) typing() bool { return false }
func (t *filesTab) hints() [][2]string {
	return [][2]string{{"enter", "open"}, {"⌫", "up"}, {"e", "edit"}, {"o", "open with"}, {"/", "search names + content"}, {"f", "find by name"}, {"y", "copy path"}, {"J/K", "scroll preview"}}
}

func (t *filesTab) open(m *model) tea.Cmd {
	if t.list == nil {
		t.root = m.app.Spec.Dir
		t.dir = t.root
		t.list = newGrid("files", col("NAME", 0), rcol("SIZE", 8), rcol("MODIFIED", 9))
		t.list.sortDefault(0, false)
		t.results = newGrid("files-hits", col("FILE", 0), col("MATCH", 0))
		t.idx.root = t.root
	}
	return t.refresh(m)
}

func (t *filesTab) refresh(*model) tea.Cmd {
	dir := t.dir
	return func() tea.Msg {
		es, err := os.ReadDir(dir)
		return filesMsg{dir: dir, entries: es, err: err}
	}
}

func (t *filesTab) update(m *model, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case filesMsg:
		if msg.dir != t.dir {
			return nil
		}
		t.err = ""
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		t.setRows(msg.entries)
	case foundMsg:
		if msg.seq == t.seq {
			t.hits, t.found = msg.hits, true
			rows := make([]grow, len(msg.hits))
			for i, h := range msg.hits {
				name, text := h.path, h.text
				if h.line > 0 {
					name += ":" + strconv.Itoa(h.line)
				}
				rows[i] = grow{id: strconv.Itoa(i), cells: []string{name, text}}
			}
			t.results.sortBy = -1
			t.results.set(rows)
		}
	case tea.KeyPressMsg:
		if t.query != "" {
			return t.searchKey(m, msg)
		}
		r, ok := t.list.current()
		switch msg.String() {
		case "enter", "right", "l":
			if ok {
				return t.enter(m, r.id)
			}
		case "backspace", "left", "h":
			return t.up(m)
		case "e":
			if ok && !strings.HasSuffix(r.id, "/") {
				return openFileAt(r.id, 1)
			}
		case "o":
			if ok {
				return m.openWith(filepath.Base(r.id), "file://"+strings.TrimSuffix(r.id, "/"))
			}
		case "y":
			if ok {
				copyText(r.id)
				m.setStatus("copied "+r.id, false)
			}
		case "/":
			return t.askSearch(m, "content")
		case "f":
			return t.askSearch(m, "name")
		case "J":
			t.scroll += 10
		case "K":
			t.scroll = max(0, t.scroll-10)
		default:
			t.list.key(msg)
		}
	}
	return nil
}

// atRoot: esc leaves the screen once the filter is cleared and the project's directory shows.
func (t *filesTab) atRoot() bool { return t.query == "" && t.filter == "" && t.dir == t.root }

func (t *filesTab) askSearch(m *model, mode string) tea.Cmd {
	label := "find file by name"
	if mode == "content" {
		label = "search file names and content"
	}
	m.ask(label, t.query, func(v string) tea.Cmd {
		t.query, t.mode, t.hits, t.found, t.seq = v, mode, nil, false, t.seq+1
		if v == "" {
			return nil
		}
		t.idx.forget()
		seq, ix := t.seq, &t.idx
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			hits := searchNames(ix.list(ctx), v)
			if mode == "content" {
				hits = append(hits, ix.searchContent(ctx, v)...)
			}
			return foundMsg{seq: seq, hits: hits}
		}
	})
	return nil
}

func (t *filesTab) hit() (fileHit, bool) {
	r, ok := t.results.current()
	if !ok {
		return fileHit{}, false
	}
	i, _ := strconv.Atoi(r.id)
	if i >= len(t.hits) {
		return fileHit{}, false
	}
	h := t.hits[i]
	h.path = filepath.Join(t.root, h.path)
	return h, true
}

// searchKey drives the hit list: enter opens the editor on the line, esc goes back to the folders.
func (t *filesTab) searchKey(m *model, k tea.KeyPressMsg) tea.Cmd {
	h, ok := t.hit()
	switch k.String() {
	case "esc", "backspace", "left", "h":
		t.query, t.hits, t.found = "", nil, false
	case "enter", "right", "l", "e":
		if ok {
			return openFileAt(h.path, max(h.line, 1))
		}
	case "g":
		if ok {
			t.dir, t.query, t.hits = filepath.Dir(h.path), "", nil
			t.list.want = h.path
			return t.refresh(m)
		}
	case "y":
		if ok {
			copyText(h.path)
			m.setStatus("copied "+h.path, false)
		}
	case "/":
		return t.askSearch(m, "content")
	case "f":
		return t.askSearch(m, "name")
	default:
		t.results.key(k)
	}
	return nil
}

func (t *filesTab) enter(m *model, path string) tea.Cmd {
	if !strings.HasSuffix(path, "/") {
		return openFileAt(path, 1)
	}
	t.dir, t.filter, t.list.sel, t.list.offset = filepath.Clean(path), "", 0, 0
	return t.refresh(m)
}

func (t *filesTab) up(m *model) tea.Cmd {
	if t.filter != "" {
		t.filter = ""
		return t.refresh(m)
	}
	if t.dir == t.root {
		return nil
	}
	from := t.dir
	t.dir = filepath.Dir(t.dir)
	cmd := t.refresh(m)
	t.list.sel, t.list.offset = 0, 0
	t.list.want = from + "/"
	return cmd
}

func (t *filesTab) setRows(es []os.DirEntry) {
	var rows []grow
	for _, e := range es {
		if e.Name() == ".git" || t.filter != "" && !strings.Contains(strings.ToLower(e.Name()), strings.ToLower(t.filter)) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path, name, size, sizeKey := filepath.Join(t.dir, e.Name()), e.Name(), bytesText(info.Size()), float64(info.Size())
		if info.IsDir() {
			path, name, size, sizeKey = path+"/", "▸ "+name+"/", "", -1
		}
		age := time.Since(info.ModTime())
		rows = append(rows, grow{id: path, cells: []string{name, size, shortAge(age)}, keys: []any{dirFirst(info.IsDir(), e.Name()), sizeKey, age.Seconds()}})
	}
	t.list.set(rows)
}

// dirFirst sorts folders before files by name.
func dirFirst(dir bool, name string) string {
	if dir {
		return "0" + strings.ToLower(name)
	}
	return "1" + strings.ToLower(name)
}

func (t *filesTab) view(m *model, w, h int) string {
	lw := m.paneSize("files", splitGeo{total: w, minA: 24, minB: 30}, min(max(w/3, 36), 70), 0, 0, h)
	rel, _ := filepath.Rel(t.root, t.dir)
	title := "files · " + filepath.Base(t.root)
	if rel != "." {
		title += "/" + rel
	}
	if t.filter != "" {
		title += " · filter " + t.filter
	}
	body := t.list.view(m, 1, 1, lw-2, h-2, true)
	if t.query != "" {
		title = fmt.Sprintf("%s · %q · %d hits", map[string]string{"content": "search", "name": "find"}[t.mode], t.query, len(t.hits))
		body = t.results.view(m, 1, 1, lw-2, h-2, true)
		switch {
		case !t.found:
			body = sDim.Render("searching…")
		case len(t.hits) == 0:
			body = sDim.Render("no file or line matches · / or f searches again, esc goes back")
		}
	}
	if t.err != "" {
		body = sRed.Render(wrap(t.err, lw-4))
	}
	left := panel(title, body, lw, h, true)
	r, ok := t.list.current()
	line := 0
	if t.query != "" {
		var h fileHit
		if h, ok = t.hit(); ok {
			r, line = grow{id: h.path}, h.line
		}
	}
	pt, preview := "preview", sDim.Render("select a file")
	if ok {
		pt = filepath.Base(strings.TrimSuffix(r.id, "/"))
		if key := r.id + fmt.Sprint(w-lw, line); t.previewFor != key {
			t.preview, t.previewFor, t.scroll = previewFile(r.id, w-lw-4), key, max(0, line-3)
		}
		lines := strings.Split(t.preview, "\n")
		t.scroll = min(t.scroll, max(0, len(lines)-1))
		preview = strings.Join(lines[t.scroll:], "\n")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, panel(pt, preview, w-lw, h, false))
}

func (t *filesTab) click(m *model, h hit) tea.Cmd {
	if t.list.click(h) && h.double {
		if r, ok := t.list.current(); ok {
			return t.enter(m, r.id)
		}
	}
	return nil
}

// previewFile is a folder's entries, a Markdown file rendered, or the start of a text file.
func previewFile(path string, w int) string {
	if strings.HasSuffix(path, "/") {
		es, err := os.ReadDir(path)
		if err != nil {
			return sRed.Render(err.Error())
		}
		var names []string
		for _, e := range es {
			n := e.Name()
			if e.IsDir() {
				n += "/"
			}
			names = append(names, n)
		}
		slices.Sort(names)
		return sDim.Render(fmt.Sprintf("%d entries", len(names))) + "\n" + strings.Join(names, "\n")
	}
	f, err := os.Open(path)
	if err != nil {
		return sRed.Render(err.Error())
	}
	defer f.Close()
	buf := make([]byte, 64<<10)
	n, _ := f.Read(buf)
	buf = buf[:n]
	if bytes.IndexByte(buf, 0) >= 0 || !utf8.Valid(buf[:max(0, n-4)]) {
		return sDim.Render("binary file: o opens it with an app")
	}
	text := strings.ReplaceAll(string(buf), "\t", "    ")
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return renderMarkdown(text, w)
	case ".yaml", ".yml":
		return colorYAML(text)
	}
	return text
}
