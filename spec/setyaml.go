package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SetYAML sets path to value in a YAML file, creating the file and mappings on the way and keeping
// the rest of it, comments included.
func SetYAML(file string, path []string, value any) error {
	var doc yaml.Node
	raw, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	n := doc.Content[0]
	for i, k := range path {
		if n.Kind != yaml.MappingNode {
			return fmt.Errorf("%s: %s is not a mapping", file, strings.Join(path[:i], "."))
		}
		var next *yaml.Node
		for j := 0; j+1 < len(n.Content); j += 2 {
			if n.Content[j].Value == k {
				next = n.Content[j+1]
			}
		}
		if i == len(path)-1 {
			var v yaml.Node
			if err := v.Encode(value); err != nil {
				return err
			}
			if next != nil {
				*next = v
			} else {
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &v)
			}
			break
		}
		if next == nil || next.Kind == yaml.ScalarNode && next.Tag == "!!null" {
			if next == nil {
				next = &yaml.Node{}
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, next)
			}
			*next = yaml.Node{Kind: yaml.MappingNode}
		}
		n = next
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644)
}
