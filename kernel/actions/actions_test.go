package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goxang/jig/kernel/audit"
	"github.com/goxang/jig/kernel/identity"
	"github.com/goxang/jig/kernel/jobs"
	"github.com/goxang/jig/kernel/policy"
	"github.com/goxang/jig/protocol"
)

func TestRunGoesThroughPolicyJobAndAudit(t *testing.T) {
	reg := NewRegistry()
	ran := 0
	_ = reg.Add(Action{Name: "runtime.restart", Effect: protocol.EffectWrite, Run: func(ctx context.Context, a protocol.Args) (*protocol.RunResult, error) {
		ran++
		jobs.Out(ctx).Write([]byte("restarting\n"))
		return &protocol.RunResult{Message: "restarted"}, nil
	}})
	log := filepath.Join(t.TempDir(), "audit.jsonl")
	asked := false
	r := &Runner{Actions: reg, Jobs: &jobs.Manager{}, Audit: audit.Open(log),
		Envs: map[string]policy.Env{"lt": {Protected: true}, "prod": {Readonly: true}},
		Ask:  func(context.Context, policy.Decision, string) error { asked = true; return ErrDeclined }}
	me := identity.Local()

	j, res, err := r.Run(context.Background(), Call{Subject: me, Env: "local", Action: "runtime.restart"})
	if err != nil || res.Message != "restarted" || j.Output()[0] != "restarting" || j.Running() {
		t.Fatalf("local run: %v %v", res, err)
	}
	if _, _, err := r.Run(context.Background(), Call{Subject: me, Env: "lt", Action: "runtime.restart"}); !errors.Is(err, ErrDeclined) || !asked {
		t.Fatalf("protected: %v asked=%v", err, asked)
	}
	var pe *protocol.Error
	if _, _, err := r.Run(context.Background(), Call{Subject: me, Env: "prod", Action: "runtime.restart"}); !errors.As(err, &pe) || pe.Code != protocol.CodeDenied {
		t.Fatalf("readonly: %v", err)
	}
	if _, _, err := r.Run(context.Background(), Call{Subject: identity.AI(me), Env: "local", Action: "runtime.restart"}); !errors.As(err, &pe) {
		t.Fatalf("ai: %v", err)
	}
	if ran != 1 {
		t.Fatalf("ran %d times", ran)
	}
	b, _ := os.ReadFile(log)
	if n := strings.Count(string(b), "\n"); n != 4 {
		t.Fatalf("audited %d entries:\n%s", n, b)
	}
}
