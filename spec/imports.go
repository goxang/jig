package spec

import (
	"fmt"
	"github.com/goxang/jig/kernel/config"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Importer turns another tool's service file into jig services; paths in them are relative to dir,
// the project's directory.
type Importer func(path, dir string) (map[string]*Service, error)

var importers = map[string]Importer{"godev": importGodev}

func RegisterImporter(name string, f Importer) { importers[name] = f }

func Importers() []string { return keys(importers) }

// importFragment runs one `imports:` entry as a configuration layer below jig.yaml, so a service
// jig.yaml also defines is the imported one with jig.yaml's keys over it. A workload a jig.yaml
// service already claims under another name is left out.
func importFragment(kind, path, dir string, root *yaml.Node) (*config.Doc, error) {
	f, ok := importers[kind]
	if !ok {
		return nil, fmt.Errorf("unknown import %q; have %v", kind, Importers())
	}
	path = os.ExpandEnv(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	svcs, err := f(path, dir)
	if err != nil {
		return nil, fmt.Errorf("import %s %s: %w", kind, path, err)
	}
	var own map[string]*Service
	if n := config.Value(root, "services"); n != nil {
		_ = n.Decode(&own)
	}
	claimed := map[string]string{}
	for n, s := range own {
		if s != nil {
			claimed[s.workload(n)] = n
		}
	}
	out := map[string]*Service{}
	for n, s := range svcs {
		if by, ok := claimed[s.workload(n)]; ok && by != n {
			continue
		}
		out[n] = s
	}
	var frag yaml.Node
	if err := frag.Encode(map[string]any{"services": out}); err != nil {
		return nil, err
	}
	prune(&frag)
	return config.FromNode(&frag, path, "import "+kind), nil
}

// prune drops the zero values encoding a Service writes, so they cannot hide a template's keys.
func prune(n *yaml.Node) bool {
	switch n.Kind {
	case yaml.MappingNode:
		out := n.Content[:0]
		for i := 0; i+1 < len(n.Content); i += 2 {
			if !prune(n.Content[i+1]) {
				out = append(out, n.Content[i], n.Content[i+1])
			}
		}
		n.Content = out
		return len(out) == 0
	case yaml.SequenceNode:
		for _, c := range n.Content {
			prune(c)
		}
		return len(n.Content) == 0
	case yaml.ScalarNode:
		return n.Tag == "!!null" || n.Value == "" && n.Style == 0 || n.Tag == "!!bool" && n.Value == "false" || n.Tag == "!!int" && n.Value == "0"
	}
	return false
}

// workload is the Kubernetes workload service name stands for, kind/name in lower case: k8s.workload,
// else the Deployment of that name.
func (s *Service) workload(name string) string {
	var sec struct {
		Workload string `yaml:"workload"`
	}
	_, _ = s.Section("k8s", &sec)
	w := strings.ToLower(sec.Workload)
	if w == "" {
		w = name
	}
	if !strings.Contains(w, "/") {
		w = "deployment/" + w
	}
	return w
}

// ImportPaths are the absolute paths of the project's imports of these kinds, in file order.
func (p *Project) ImportPaths(kinds ...string) []string {
	var out []string
	for _, imp := range p.Imports {
		for _, k := range kinds {
			if path, ok := imp[k]; ok {
				if !filepath.IsAbs(path) {
					path = filepath.Join(p.Dir, path)
				}
				out = append(out, path)
			}
		}
	}
	return out
}

func mappingValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func importGodev(path, _ string) (map[string]*Service, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Services map[string]struct {
			Path    string            `yaml:"path"`
			Command []string          `yaml:"command"`
			Args    []string          `yaml:"args"`
			Env     map[string]string `yaml:"env"`
			Group   []string          `yaml:"group"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	out := map[string]*Service{}
	for n, g := range f.Services {
		s := &Service{Name: n, Groups: g.Group, Env: g.Env, Role: RoleApp}
		switch {
		case len(g.Command) > 0:
			s.Run = &Run{Command: g.Command}
		case g.Path != "":
			s.Build = &Build{Go: g.Path}
			s.Run = &Run{Args: g.Args}
		}
		sort.Strings(s.Groups)
		out[n] = s
	}
	return out, nil
}
