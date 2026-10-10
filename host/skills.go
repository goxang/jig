package host

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill is a SKILL.md: its front matter's name and description, always shown to the AI, and the
// body, loaded only when asked for.
type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Plugin      string `yaml:"-"`
	Body        string `yaml:"-"`
}

// ParseSkill reads a SKILL.md; fallback names it when its front matter does not.
func ParseSkill(raw, fallback string) Skill {
	s := Skill{Name: fallback, Body: raw}
	if rest, ok := strings.CutPrefix(raw, "---\n"); ok {
		if head, body, ok := strings.Cut(rest, "\n---"); ok {
			_ = yaml.Unmarshal([]byte(head), &s)
			s.Body = strings.TrimLeft(body, "-\n")
		}
	}
	if s.Name == "" {
		s.Name = fallback
	}
	return s
}

// Skills are the skills of every plugin, from the skills: directories their manifests list.
func (h *Host) Skills() []Skill {
	var out []Skill
	for _, p := range h.Plugins() {
		for _, d := range p.Manifest.Skills {
			path := d
			if !filepath.IsAbs(path) {
				path = filepath.Join(p.Manifest.Dir, d)
			}
			if !strings.HasSuffix(path, ".md") {
				path = filepath.Join(path, "SKILL.md")
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			s := ParseSkill(string(raw), p.Name+"-"+filepath.Base(filepath.Dir(path)))
			s.Plugin = p.Name
			out = append(out, s)
		}
	}
	return out
}
