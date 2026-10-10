package spec

import (
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// A stand-in for the godev plugin importer, so tests load imports without plugins.
func init() { RegisterImporter("godev", importGodev) }

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
