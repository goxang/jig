// Package actions runs named verbs: each goes through policy, runs as a job and, unless it only
// reads, lands in the audit log.
package actions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/goxang/jig/kernel/audit"
	"github.com/goxang/jig/kernel/identity"
	"github.com/goxang/jig/kernel/jobs"
	"github.com/goxang/jig/kernel/policy"
	"github.com/goxang/jig/protocol"
)

type Func func(ctx context.Context, args protocol.Args) (*protocol.RunResult, error)

type Action struct {
	Name   string
	Effect string
	Help   string
	Args   protocol.Schema
	Run    Func
	// Plan, when set, says what Run would do without doing it.
	Plan Func
}

type Registry struct {
	mu sync.RWMutex
	m  map[string]Action
}

func NewRegistry() *Registry { return &Registry{m: map[string]Action{}} }

func (r *Registry) Add(a Action) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[a.Name]; ok {
		return fmt.Errorf("action %s is already registered", a.Name)
	}
	r.m[a.Name] = a
	return nil
}

func (r *Registry) Get(name string) (Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.m[name]
	return a, ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.m))
	for n := range r.m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ErrDeclined is a confirm or reauth the user said no to.
var ErrDeclined = errors.New("declined")

type Runner struct {
	Actions *Registry
	Policy  *policy.Config
	Envs    map[string]policy.Env
	Jobs    *jobs.Manager
	// Audit, when set, logs every action that is not a read.
	Audit *audit.Log
	// Ask settles a confirm or reauth decision: true to go ahead.
	Ask func(ctx context.Context, d policy.Decision, why string) bool
}

type Call struct {
	Subject identity.Subject
	Env     string
	Action  string
	Args    protocol.Args
	DryRun  bool
}

// Run runs c as a job and waits for it.
func (r *Runner) Run(ctx context.Context, c Call) (*jobs.Job, *protocol.RunResult, error) {
	a, ok := r.Actions.Get(c.Action)
	if !ok {
		return nil, nil, &protocol.Error{Code: protocol.CodeNotFound, Message: "no action " + c.Action}
	}
	if c.DryRun {
		if a.Plan == nil {
			return nil, nil, &protocol.Error{Code: protocol.CodeUnavailable, Message: c.Action + " has no plan"}
		}
		res, err := a.Plan(ctx, c.Args)
		return nil, res, err
	}
	d, why := r.Policy.Decide(policy.Request{Subject: c.Subject, Action: c.Action, Effect: a.Effect, Env: c.Env}, r.Envs)
	entry := audit.Entry{Subject: c.Subject, Action: c.Action, Args: c.Args, Effect: a.Effect, Env: c.Env, Decision: string(d)}
	var err error
	switch d {
	case policy.Deny, policy.Approve:
		err = &protocol.Error{Code: protocol.CodeDenied, Message: fmt.Sprintf("%s: %s (%s)", c.Action, d, why)}
	case policy.Confirm, policy.Reauth:
		if r.Ask == nil || !r.Ask(ctx, d, why) {
			err = ErrDeclined
		}
	}
	if err != nil {
		entry.Error = err.Error()
		return nil, nil, errors.Join(err, r.log(a, entry))
	}
	ctx, j := r.Jobs.Start(ctx, c.Action)
	res, err := a.Run(ctx, c.Args)
	j.Finish(err)
	if err != nil {
		entry.Error = err.Error()
	} else if res != nil {
		entry.Result = res.Message
	}
	return j, res, errors.Join(err, r.log(a, entry))
}

func (r *Runner) log(a Action, e audit.Entry) error {
	if r.Audit == nil || a.Effect == protocol.EffectRead {
		return nil
	}
	if err := r.Audit.Append(e); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}
