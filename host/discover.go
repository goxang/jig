package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goxang/jig/kernel/config"
	"github.com/goxang/jig/spec"
)

func init() {
	spec.PluginDefaults = func(dir string, cfg map[string]*spec.Plugin) []*config.Doc {
		ms, _ := Discover(dir, cfg)
		var out []*config.Doc
		for _, m := range ms {
			out = append(out, m.Fragment)
		}
		return out
	}
}

func xdg(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

func ConfigDir() string { return filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), "jig") }
func StoreDir() string  { return filepath.Join(xdg("XDG_DATA_HOME", ".local/share"), "jig", "plugins") }
func StateDir() string  { return filepath.Join(xdg("XDG_STATE_HOME", ".local/state"), "jig") }
func CacheDir() string  { return filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), "jig") }

// Discover finds the project's plugins. A later scope replaces an earlier one of the same name: what
// jig.lock pins, then user plugins, then project plugins (under workspace trust), then plugins.<name>.path.
func Discover(dir string, cfg map[string]*spec.Plugin) (ms []*Manifest, warnings []string) {
	found := map[string]*Manifest{}
	add := func(d string) {
		m, err := LoadManifest(d)
		if err != nil {
			if !os.IsNotExist(err) {
				warnings = append(warnings, err.Error())
			}
			return
		}
		warnings = append(warnings, m.Warnings...)
		if !HostSupports(m.Jig) {
			warnings = append(warnings, fmt.Sprintf("plugin %s needs jig %s; this is %s", m.Name, m.Jig, Version))
			return
		}
		found[m.Name] = m
	}
	resolve := func(p string) string {
		if rest, ok := strings.CutPrefix(p, "~/"); ok {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, rest)
		}
		if !filepath.IsAbs(p) {
			return filepath.Join(dir, p)
		}
		return p
	}
	lock, err := ReadLock(dir)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s: %v", LockFile, err))
		lock = &Lock{}
	}
	trusted := Trusted(dir)
	if !trusted && (len(lock.Plugins) > 0 || exists(filepath.Join(dir, ".jig", "plugins"))) {
		warnings = append(warnings, "project plugins stay off until the workspace is trusted (jig trust)")
	}
	for _, n := range lock.Names() {
		if !trusted {
			break
		}
		l := lock.Plugins[n]
		if isPathSource(l.Source) {
			add(resolve(l.Source))
		} else if d := storePath(n, l.Version); exists(d) {
			add(d)
		} else {
			warnings = append(warnings, fmt.Sprintf("plugin %s %s is locked but not installed (any jig command installs it)", n, l.Version))
		}
	}
	scan := func(root string) {
		ents, _ := os.ReadDir(root)
		for _, e := range ents {
			if e.IsDir() {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	scan(filepath.Join(ConfigDir(), "plugins"))
	if trusted {
		scan(filepath.Join(dir, ".jig", "plugins"))
	}
	for name, c := range cfg {
		if trusted && c != nil && c.Path != "" {
			p := resolve(c.Path)
			add(p)
			if m := found[name]; m == nil || m.Dir != filepath.Clean(p) {
				warnings = append(warnings, fmt.Sprintf("plugins.%s.path: no plugin named %s in %s", name, name, p))
			}
		}
	}
	for name, m := range found {
		if c := cfg[name]; c != nil && c.Off {
			continue
		}
		ms = append(ms, m)
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
	return ms, warnings
}

func canon(v string) string {
	if len(v) > 0 && v[0] != 'v' {
		return "v" + v
	}
	return v
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// Trusted says whether project-local plugins and auto-install may run in dir.
func Trusted(dir string) bool {
	if os.Getenv("JIG_TRUST") == "project" {
		return true
	}
	t := readTrust()
	abs, _ := filepath.Abs(dir)
	return t[abs]
}

func readTrust() map[string]bool {
	t := map[string]bool{}
	if b, err := os.ReadFile(filepath.Join(ConfigDir(), "trust.json")); err == nil {
		_ = json.Unmarshal(b, &t)
	}
	return t
}

// Trust records that dir's plugins may run.
func Trust(dir string) error {
	t := readTrust()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	t[abs] = true
	b, _ := json.MarshalIndent(t, "", "  ")
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(ConfigDir(), "trust.json"), b, 0o600)
}

func writeAtomic(path string, b []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
