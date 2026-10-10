package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/mod/semver"

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

// Discover finds the project's plugins. A later scope replaces an earlier one of the same name:
// installed store, then user plugins, then project plugins (under workspace trust), then plugins.<name>.path.
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
		found[m.Name] = m
	}
	stored, _ := os.ReadDir(StoreDir())
	for _, e := range stored {
		if v := installed(e.Name(), cfg); v != "" {
			add(filepath.Join(StoreDir(), e.Name(), v))
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
	if local := filepath.Join(dir, ".jig", "plugins"); exists(local) {
		if Trusted(dir) {
			scan(local)
		} else {
			warnings = append(warnings, fmt.Sprintf("%s: project plugins skipped until the workspace is trusted (jig trust, or JIG_TRUST=project)", local))
		}
	}
	for name, c := range cfg {
		if c != nil && c.Path != "" {
			p := c.Path
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
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

// installed picks the version of an installed plugin: the pinned one, else the highest.
func installed(name string, cfg map[string]*spec.Plugin) string {
	vs, _ := os.ReadDir(filepath.Join(StoreDir(), name))
	if c := cfg[name]; c != nil && c.Version != "" {
		for _, v := range vs {
			if v.Name() == c.Version || v.Name() == "v"+c.Version {
				return v.Name()
			}
		}
	}
	best := ""
	for _, v := range vs {
		if semver.IsValid(canon(v.Name())) && (best == "" || semver.Compare(canon(v.Name()), canon(best)) > 0) {
			best = v.Name()
		}
	}
	return best
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
