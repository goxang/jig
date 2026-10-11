package cli

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAppendToEmptyBlock(t *testing.T) {
	out := appendToBlock("name: x\nservices: {}\n", "services", "  a:\n    image: i\n")
	var doc struct{ Services map[string]any }
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil || doc.Services["a"] == nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
