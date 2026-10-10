package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStepHelp(t *testing.T) {
	var task Task
	src := `
help: x
steps:
  # wipe the history tables
  - jig db query "TRUNCATE"
  - |
    # restart every redis
    for r in a b; do jig restart $r; done
  - jig up
`
	if err := yaml.Unmarshal([]byte(src), &task); err != nil {
		t.Fatal(err)
	}
	want := []string{"wipe the history tables", "restart every redis", ""}
	for i, w := range want {
		if task.StepHelp[i] != w {
			t.Errorf("step %d: %q, want %q", i, task.StepHelp[i], w)
		}
	}
}
