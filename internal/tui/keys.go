package tui

import (
	"cmp"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"

	"github.com/goxang/jig/host"
)

// keyPresets map a key to a global command; keys left out go to the screen. Keys every preset
// shares (q, @, ?, tab, digits, ...) stay in the global switch.
var keyPresets = map[string]map[string]string{
	"jig": {":": "palette", "ctrl+k": "search", "ctrl+e": "env", ".": "menu", "g": "go", "T": "task"},
	"rig": {"S": "save", "ctrl+e": "envinfo", "ctrl+k": "kubeconfig", "E": "env", "T": "task", "N": "namespace", ":": "palette"},
}

// goKeys are the g prefix: a screen by name, or a key the screen takes for its selection.
var goKeys = [][3]string{
	{"l", "logs", "Logs"}, {"m", "metrics", "Metrics"}, {"t", "traces", "Traces"}, {"d", "data", "Data"},
	{"s", "services", "Services"}, {"g", "top", ""},
}

// preset is keys: from the project (.jig/local.yaml), else ~/.config/jig/config.yaml, else jig.
func (m *model) preset() string {
	if k := m.app.Spec.Keys; k != "" {
		return k
	}
	dir, _ := os.UserConfigDir()
	var c struct{ Keys string }
	if b, err := os.ReadFile(filepath.Join(dir, "jig", "config.yaml")); err == nil {
		_ = yaml.Unmarshal(b, &c)
	}
	return cmp.Or(c.Keys, "jig")
}

// globalKey runs the preset's command for s, or a view's own key; ok is false when neither has it.
func (m *model) globalKey(s string) (tea.Cmd, bool) {
	if m.prefix != "" {
		p := m.prefix
		m.prefix = ""
		return m.goKey(s), p == "g"
	}
	if c, ok := keyPresets[m.preset()][s]; ok {
		return m.command(c), true
	}
	for i, t := range m.tabs {
		if v, ok := t.(*viewTab); ok && v.v.Key == s {
			return m.jump(i), true
		}
	}
	return nil, false
}

func (m *model) goKey(s string) tea.Cmd {
	for _, g := range goKeys {
		if g[0] != s {
			continue
		}
		if g[2] == "" {
			return m.tabs[m.active].update(m, keyOf("home"))
		}
		// the services screen takes l and m itself, with its selection
		if _, ok := m.tabs[m.active].(*servicesTab); ok && (s == "l" || s == "m") {
			return m.tabs[m.active].update(m, keyOf(s))
		}
		return m.command("open " + g[2])
	}
	return nil
}

// whichKey is the box of the pending prefix's keys.
func (m *model) whichKey() string {
	var hs [][2]string
	for _, g := range goKeys {
		hs = append(hs, [2]string{"g " + g[0], g[1]})
	}
	return keyHints(hs)
}

// command runs a palette line: the CLI's grammar, with what the UI does itself handled here and
// the rest run as `jig <line>` with its output in a box.
func (m *model) command(line string) tea.Cmd {
	f := strings.Fields(line)
	if len(f) == 0 {
		return nil
	}
	verb, args := f[0], f[1:]
	switch verb {
	case "palette":
		m.ask(":", "", m.command)
		return nil
	case "search":
		m.search()
		return nil
	case "menu":
		m.menu()
		return nil
	case "go":
		m.prefix = "g"
		return nil
	case "q", "quit":
		return m.quit()
	case "reload":
		return m.reload()
	case "save":
		id, err := m.saveSession()
		if err != nil {
			m.setStatus("save session: "+err.Error(), true)
		} else {
			m.setStatus("session "+id+" saved: jig opens on it from now on, as you leave it (jig --fresh starts clean)", false)
		}
		return nil
	case "env":
		m.pickEnv()
		return nil
	case "envinfo":
		return m.fetchEnvInfo()
	case "ns", "namespace":
		return m.pickNamespace()
	case "kubeconfig":
		return m.fetchKubeconfig(cmp.Or(m.kubeEnv, m.app.Env.Name))
	case "theme":
		m.pickTheme()
		return nil
	case "task":
		if len(args) == 0 {
			m.pickTask("")
			return nil
		}
		if t, ok := m.app.Tasks()[args[0]]; ok {
			m.askTaskArgs(args[0], t.Args, nil)
			return nil
		}
	case "open":
		want := strings.ToLower(strings.Join(args, " "))
		for i, t := range m.tabs {
			if strings.ToLower(t.name()) == want {
				return m.jump(i)
			}
		}
		m.setStatus("no screen "+want, true)
		return nil
	case "service":
		if s := m.servicesTab(); s != nil && len(args) > 0 {
			s.focusService(m, args[0])
			return m.jump(m.tabIndex(s))
		}
	case "start", "stop", "restart", "deploy", "build", "logs":
		if s := m.servicesTab(); s != nil && len(args) > 0 {
			return s.ops(m, map[string]string{"start": "s", "stop": "x", "restart": "r", "deploy": "d", "build": "b", "logs": "l"}[verb], args)
		}
	}
	return m.runJig(line, f)
}

func (m *model) servicesTab() *servicesTab {
	for _, t := range m.tabs {
		if s, ok := t.(*servicesTab); ok {
			return s
		}
	}
	return nil
}

func (m *model) tabIndex(t tab) int {
	for i, x := range m.tabs {
		if x == t {
			return i
		}
	}
	return m.active
}

// runJig runs the line as a jig command in this project and environment.
func (m *model) runJig(line string, args []string) tea.Cmd {
	exe, err := os.Executable()
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	a, gen := m.app, m.gen
	return m.do(":"+line, func(ctx context.Context) error {
		c := exec.CommandContext(ctx, exe, append([]string{"-f", a.Spec.File, "-e", a.Env.Name}, args...)...)
		c.Dir = a.Spec.Dir
		c.Env = append(os.Environ(), "JIG_BRIEF=1", "NO_COLOR=1")
		out, err := c.CombinedOutput()
		text := strings.TrimRight(string(out), "\n")
		if err != nil {
			last := text
			if i := strings.LastIndex(text, "\n"); i >= 0 {
				last = text[i+1:]
			}
			return errors.New(cmp.Or(last, err.Error()))
		}
		if text != "" && program != nil {
			box := &envBox{env: ":" + line}
			for l := range strings.SplitSeq(text, "\n") {
				box.lines = append(box.lines, envLine{text: "  " + l})
			}
			program.Send(envInfoMsg{gen: gen, box: box})
		}
		return nil
	})
}

// search is ctrl+k: screens, services, tasks, actions and plugins in one filtered list; each item
// is a palette line.
func (m *model) search() {
	var items, desc []string
	add := func(line, d string) { items, desc = append(items, line), append(desc, d) }
	for _, t := range m.tabs {
		add("open "+t.name(), "screen")
	}
	for _, s := range m.app.Spec.ServiceNames() {
		add("service "+s, "service")
	}
	for _, t := range m.app.TaskNames() {
		add("task "+t, cmp.Or(m.app.TaskHelp(t), "task"))
	}
	for _, a := range m.eng().Actions() {
		if c, verb, ok := strings.Cut(a, "."); ok {
			add("do "+c+" "+verb, "action")
		}
	}
	ms, _ := host.Discover(m.app.Spec.Dir, m.app.Spec.Plugins)
	for _, p := range ms {
		add("plugin info "+p.Name, cmp.Or(p.Description, "plugin"))
	}
	m.pick("search", items, desc, 0, false, func(chosen []string) tea.Cmd {
		if len(chosen) == 0 {
			return nil
		}
		return m.command(chosen[0])
	})
}

// menu is . or right-click: the screen's keys as a list, each one pressed when chosen.
func (m *model) menu() {
	t := m.tabs[m.active]
	var items, desc []string
	for _, h := range t.hints() {
		if strings.ContainsAny(h[0], " /↑↓←→") && h[0] != "/" {
			continue
		}
		items, desc = append(items, h[0]), append(desc, h[1])
	}
	if len(items) == 0 {
		return
	}
	m.pick(t.name(), items, desc, 0, false, func(chosen []string) tea.Cmd {
		if len(chosen) == 0 {
			return nil
		}
		return t.update(m, keyOf(chosen[0]))
	})
}

// keyFor is the preset's key for a command, or "" when it has none.
func (m *model) keyFor(command string) string {
	for k, c := range keyPresets[m.preset()] {
		if c == command {
			return k
		}
	}
	return ""
}

// commandAsking runs a palette line, first asking for the ${ask} in it.
func (m *model) commandAsking(label, line string) tea.Cmd {
	if !strings.Contains(line, "${ask}") {
		return m.command(line)
	}
	m.ask(label, "", func(v string) tea.Cmd { return m.command(strings.ReplaceAll(line, "${ask}", v)) })
	return nil
}

// commandMsg runs a palette line on the UI loop, as a confirmed answer does.
type commandMsg string
