package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goxang/jig/api"
	"github.com/goxang/jig/kernel/identity"
	"github.com/goxang/jig/protocol"
)

func openLocal(t *testing.T, env string) *Local {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "jig.yaml")
	os.WriteFile(file, []byte(`project: l
default: dev
environments:
  dev: { runtime: fake }
  shared: { runtime: fake, protected: true }
services:
  api: { run: { command: [sleep, "60"] }, depends_on: [db] }
  db: { role: infra, run: { command: [sleep, "60"] } }
tasks:
  hi: [echo hello]
`), 0o644)
	a, err := Open(file, env)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return NewLocal(a, nil)
}

func TestLocalRunsTasksThroughPolicy(t *testing.T) {
	ctx := context.Background()
	l := openLocal(t, "dev")
	j, _, err := l.Run(ctx, "task.hi", nil, api.RunOpts{})
	if err != nil || !strings.Contains(strings.Join(j.Output(), "\n"), "hello") {
		t.Fatalf("task: %v %v", err, j.Output())
	}
	ai := identity.AI(l.Session(ctx))
	var pe *protocol.Error
	if _, _, err := l.Run(ctx, "task.hi", nil, api.RunOpts{As: &ai}); !errors.As(err, &pe) || pe.Code != protocol.CodeDenied {
		t.Fatalf("an AI write ran: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(l.App.StateDir(), "audit.jsonl"))
	if strings.Count(string(b), "\n") != 2 {
		t.Fatalf("audit:\n%s", b)
	}

	p := openLocal(t, "shared")
	if _, _, err := p.Run(ctx, "task.hi", nil, api.RunOpts{}); !errors.Is(err, ErrProtected) {
		t.Fatalf("protected without a yes: %v", err)
	}
	p.App.Confirmed = true
	if _, _, err := p.Run(ctx, "task.hi", nil, api.RunOpts{}); err != nil {
		t.Fatalf("protected with a yes: %v", err)
	}
}

func TestLocalCatalogSearchResolve(t *testing.T) {
	ctx := context.Background()
	l := openLocal(t, "dev")
	hits, _ := l.Search(ctx, "AP")
	if len(hits) != 1 || hits[0].Ref != "svc://api" {
		t.Fatalf("hits %v", hits)
	}
	r, err := l.Resolve(ctx, "svc://api")
	if err != nil || r.Kind != "service" || len(r.Links) != 1 || r.Links[0].Ref != "svc://db" {
		t.Fatalf("resolve %+v %v", r, err)
	}
	f, err := l.Query(ctx, "services.status", nil)
	if err != nil || f.Columns[0][0] != "api" {
		t.Fatalf("status %v %v", f, err)
	}
}
