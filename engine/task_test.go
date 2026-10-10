package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const taskFile = `
project: t
default: test
services:
  db: { role: infra }
environments:
  test: { runtime: { type: fake } }
tasks:
  greet:
    steps: ['echo args=$1 rest=$JIG_ARGS ref=$JIG_REF tag=$TAG']
`

func openTask(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "jig.yaml")
	if err := os.WriteFile(p, []byte(taskFile), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Open(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// NAME=value args become env vars for the task's steps instead of requiring them pre-exported,
// while plain words keep reaching the steps positionally as $1... and $JIG_ARGS.
func TestRunTaskNamedParams(t *testing.T) {
	a := openTask(t)
	var out bytes.Buffer
	if err := a.RunTask(context.Background(), "greet", []string{"JIG_REF=feature-x", "core", "TAG=v3"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "args=core rest=core ref=feature-x tag=v3") {
		t.Fatalf("output = %q", got)
	}
}
