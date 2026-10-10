package contracts

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/goxang/jig/core"
	"github.com/goxang/jig/kernel/frames"
	"github.com/goxang/jig/protocol"
	"github.com/goxang/jig/spec"
)

// Caller reaches a plugin's sources and actions by their namespaced names: the plugin host.
type Caller interface {
	Query(ctx context.Context, name string, args protocol.Args) (*protocol.Frame, error)
	Run(ctx context.Context, name string, args protocol.Args, dryRun bool) (*protocol.RunResult, error)
	Stream(ctx context.Context, name string, args protocol.Args) (<-chan protocol.StreamBatch, error)
}

// Bridge is component, served by plugin through the contract for kind, as the core interface that
// kind's built-in screens and commands use. Every call carries the component's name, so one plugin
// can serve several components.
func Bridge(kind core.Kind, c Caller, plugin, component string) (any, error) {
	b := base{c: c, plugin: plugin, comp: component}
	switch kind {
	case core.KindRuntime:
		return &runtimeBridge{b}, nil
	case core.KindBuilder:
		return &builderBridge{b}, nil
	case core.KindMetrics:
		return &metricsBridge{b}, nil
	case core.KindTracing:
		return &tracesBridge{b}, nil
	case core.KindProfiler:
		return &profilerBridge{b}, nil
	case core.KindDebugger:
		return &debuggerBridge{b}, nil
	case core.KindLogs:
		return &logsBridge{b}, nil
	case core.KindDatabase:
		return &databaseBridge{b}, nil
	case core.KindCache:
		return &cacheBridge{b}, nil
	case core.KindMessaging:
		return &queueBridge{b}, nil
	case core.KindKV:
		return &kvBridge{b}, nil
	case core.KindLoad:
		return &loadBridge{b}, nil
	case core.KindHosts:
		return &hostsBridge{b}, nil
	}
	return nil, fmt.Errorf("no contract bridges the %s kind", kind)
}

type base struct {
	c            Caller
	plugin, comp string
}

func (b base) args(a protocol.Args) protocol.Args {
	if a == nil {
		a = protocol.Args{}
	}
	a["component"] = b.comp
	return a
}

func (b base) rows(ctx context.Context, source string, a protocol.Args) ([]map[string]any, error) {
	f, err := b.c.Query(ctx, b.plugin+"."+source, b.args(a))
	if err != nil {
		return nil, err
	}
	return frames.Rows(f), nil
}

func (b base) run(ctx context.Context, action string, a protocol.Args) (protocol.Args, error) {
	res, err := b.c.Run(ctx, b.plugin+"."+action, b.args(a), false)
	if err != nil || res == nil {
		return nil, err
	}
	return res.Outputs, nil
}

func (b base) table(ctx context.Context, source string, a protocol.Args) (core.Table, error) {
	f, err := b.c.Query(ctx, b.plugin+"."+source, b.args(a))
	if err != nil {
		return core.Table{}, err
	}
	var t core.Table
	for _, fl := range f.Fields {
		t.Columns = append(t.Columns, fl.Name)
	}
	for i := range frames.Len(f) {
		row := make([]string, len(f.Fields))
		for j, fl := range f.Fields {
			row[j] = frames.Format(fl, f.Columns[j][i])
		}
		t.Rows = append(t.Rows, row)
	}
	return t, nil
}

// command runs the argv an action answers (outputs.argv) on the caller's terminal.
func command(ctx context.Context, out protocol.Args) (*exec.Cmd, error) {
	argv := strs(out["argv"])
	if len(argv) == 0 {
		return nil, fmt.Errorf("the plugin answered no argv to run")
	}
	return exec.CommandContext(ctx, argv[0], argv[1:]...), nil
}

// ---- runtime ----

type runtimeBridge struct{ base }

func (r *runtimeBridge) Discover(ctx context.Context) ([]core.Workload, error) {
	rows, err := r.rows(ctx, "runtime.discover", nil)
	var out []core.Workload
	for _, w := range rows {
		out = append(out, core.Workload{Name: str(w["name"]), Kind: str(w["kind"]), Service: str(w["service"]), Status: core.Status{State: core.State(str(w["state"]))}})
	}
	return out, err
}

func (r *runtimeBridge) svc(ctx context.Context, action string, s *spec.Service, a protocol.Args) error {
	if a == nil {
		a = protocol.Args{}
	}
	a["service"] = s.Name
	_, err := r.run(ctx, action, a)
	return err
}

func (r *runtimeBridge) Start(ctx context.Context, s *spec.Service) error {
	return r.svc(ctx, "runtime.start", s, nil)
}
func (r *runtimeBridge) Stop(ctx context.Context, s *spec.Service) error {
	return r.svc(ctx, "runtime.stop", s, nil)
}
func (r *runtimeBridge) Restart(ctx context.Context, s *spec.Service) error {
	return r.svc(ctx, "runtime.restart", s, nil)
}
func (r *runtimeBridge) Scale(ctx context.Context, s *spec.Service, n int) error {
	return r.svc(ctx, "runtime.scale", s, protocol.Args{"replicas": n})
}

func (r *runtimeBridge) Deploy(ctx context.Context, s *spec.Service, rel core.Release) error {
	a := protocol.Args{"image": rel.Image, "replicas": rel.Replicas}
	if rel.Env != nil {
		a["env"] = rel.Env
	}
	return r.svc(ctx, "runtime.deploy", s, a)
}

func (r *runtimeBridge) Status(ctx context.Context, s *spec.Service) (core.Status, error) {
	rows, err := r.rows(ctx, "runtime.status", protocol.Args{"services": []string{s.Name}})
	if err != nil {
		return core.Status{Service: s.Name, State: core.StateUnknown, Message: err.Error()}, err
	}
	for _, row := range rows {
		if str(row["service"]) == s.Name {
			return statusOf(row), nil
		}
	}
	return core.Status{Service: s.Name, State: core.StateAbsent}, nil
}

func statusOf(row map[string]any) core.Status {
	st := core.Status{Service: str(row["service"]), State: core.State(str(row["state"])), Ready: integer(row["ready"]),
		Desired: integer(row["desired"]), Image: str(row["image"]), Message: str(row["message"])}
	insts, _ := row["instances"].([]any)
	for _, i := range insts {
		m, _ := i.(map[string]any)
		st.Instances = append(st.Instances, core.Instance{ID: str(m["id"]), Host: str(m["host"]), IP: str(m["ip"]), State: core.State(str(m["state"])),
			Ready: m["ready"] == true, Restarts: integer(m["restarts"]), Started: when(m["started"]), CPU: number(m["cpu"]), Memory: int64(number(m["memory"]))})
	}
	return st
}

func (r *runtimeBridge) Logs(ctx context.Context, s *spec.Service, o core.LogOptions) (<-chan core.LogLine, error) {
	return r.stream(ctx, "runtime.logs", protocol.Args{"service": s.Name, "instance": o.Instance, "since": o.Since.String(), "tail": o.Tail, "follow": o.Follow})
}

func (r *runtimeBridge) Exec(ctx context.Context, s *spec.Service, o core.ExecOptions) error {
	out, err := r.run(ctx, "runtime.exec", protocol.Args{"service": s.Name, "instance": o.Instance, "command": o.Command, "tty": o.TTY})
	if err != nil {
		return err
	}
	cmd, err := command(ctx, out)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = o.Stdin, o.Stdout, o.Stderr
	return cmd.Run()
}

// stream reads log lines from a streaming source: frame rows, or bare lines.
func (b base) stream(ctx context.Context, source string, a protocol.Args) (<-chan core.LogLine, error) {
	in, err := b.c.Stream(ctx, b.plugin+"."+source, b.args(a))
	if err != nil {
		return nil, err
	}
	out := make(chan core.LogLine, 64)
	go func() {
		defer close(out)
		for batch := range in {
			var lines []core.LogLine
			for _, row := range frames.Rows(batch.Frame) {
				lines = append(lines, core.LogLine{Time: when(row["time"]), Service: str(row["service"]), Instance: str(row["instance"]), Stream: str(row["stream"]), Text: str(row["text"])})
			}
			for _, l := range batch.Lines {
				lines = append(lines, core.LogLine{Time: time.Now(), Text: l})
			}
			for _, l := range lines {
				select {
				case out <- l:
				case <-ctx.Done():
					return
				}
			}
			if batch.Done || batch.Error != nil {
				return
			}
		}
	}()
	return out, nil
}

// ---- builder ----

type builderBridge struct{ base }

// ponytail: build output goes to the job in ctx, not o.Out; pipe $/progress into o.Out if a caller needs it.
func (b *builderBridge) Build(ctx context.Context, s *spec.Service, o core.BuildOptions) (string, error) {
	out, err := b.run(ctx, "builder.build", protocol.Args{"service": s.Name, "tag": o.Tag, "registry": o.Registry, "push": o.Push, "dir": o.Dir})
	return str(out["image"]), err
}

// ---- observability ----

type metricsBridge struct{ base }

func (m *metricsBridge) Instant(ctx context.Context, q string) ([]core.Sample, error) {
	rows, err := m.rows(ctx, "metrics.instant", protocol.Args{"query": q})
	var out []core.Sample
	for _, r := range rows {
		out = append(out, core.Sample{Labels: labels(r), Value: number(r["value"])})
	}
	return out, err
}

func (m *metricsBridge) Range(ctx context.Context, q string, start, end time.Time, step time.Duration) ([]core.Series, error) {
	rows, err := m.rows(ctx, "metrics.range", protocol.Args{"query": q, "start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339), "step": step.String()})
	var out []core.Series
	at := map[string]int{}
	for _, r := range rows {
		k := str(r["series"])
		i, ok := at[k]
		if !ok {
			i = len(out)
			at[k] = i
			out = append(out, core.Series{Labels: labels(r)})
		}
		out[i].Points = append(out[i].Points, core.Point{T: when(r["time"]), V: number(r["value"])})
	}
	return out, err
}

func labels(r map[string]any) map[string]string {
	out := map[string]string{}
	if m, ok := r["labels"].(map[string]any); ok {
		for k, v := range m {
			out[k] = str(v)
		}
	}
	if len(out) == 0 && str(r["series"]) != "" {
		out["series"] = str(r["series"])
	}
	return out
}

type tracesBridge struct{ base }

func (t *tracesBridge) Services(ctx context.Context) ([]string, error) {
	rows, err := t.rows(ctx, "traces.services", nil)
	var out []string
	for _, r := range rows {
		out = append(out, str(r["service"]))
	}
	return out, err
}

func (t *tracesBridge) Search(ctx context.Context, q core.TraceQuery) ([]core.TraceSummary, error) {
	rows, err := t.rows(ctx, "traces.search", protocol.Args{"service": q.Service, "operation": q.Operation,
		"min_duration": q.MinDuration.String(), "lookback": q.Lookback.String(), "limit": q.Limit})
	var out []core.TraceSummary
	for _, r := range rows {
		out = append(out, core.TraceSummary{ID: str(r["id"]), Root: str(r["root"]), Start: when(r["start"]), Duration: duration(r["duration"]),
			Spans: integer(r["spans"]), Services: strs(r["services"]), Error: r["error"] == true})
	}
	return out, err
}

func (t *tracesBridge) Trace(ctx context.Context, id string) ([]core.Span, error) {
	rows, err := t.rows(ctx, "traces.get", protocol.Args{"id": id})
	var out []core.Span
	for _, r := range rows {
		tags := map[string]string{}
		if m, ok := r["tags"].(map[string]any); ok {
			for k, v := range m {
				tags[k] = str(v)
			}
		}
		out = append(out, core.Span{TraceID: cmp.Or(str(r["trace"]), id), ID: str(r["id"]), Parent: str(r["parent"]), Service: str(r["service"]),
			Name: str(r["name"]), Start: when(r["start"]), Duration: duration(r["duration"]), Error: r["error"] == true, Tags: tags})
	}
	return out, err
}

type profilerBridge struct{ base }

func (p *profilerBridge) Kinds() []string {
	rows, _ := p.rows(context.Background(), "profiles.kinds", nil)
	var out []string
	for _, r := range rows {
		out = append(out, str(r["kind"]))
	}
	return out
}

func (p *profilerBridge) Capture(ctx context.Context, r core.ProfileRequest) (core.Profile, error) {
	out, err := p.run(ctx, "profiles.capture", protocol.Args{"service": r.Service.Name, "instance": r.Instance, "kind": r.Kind, "duration": r.Duration.String(), "dir": r.OutDir})
	return core.Profile{Kind: cmp.Or(str(out["kind"]), r.Kind), Format: str(out["format"]), File: str(out["file"]), Summary: str(out["summary"])}, err
}

type debuggerBridge struct{ base }

func (d *debuggerBridge) Attach(ctx context.Context, s *spec.Service, instance string) (core.DebugSession, error) {
	out, err := d.run(ctx, "debugger.attach", protocol.Args{"service": s.Name, "instance": instance})
	if err != nil {
		return core.DebugSession{}, err
	}
	return core.DebugSession{Addr: str(out["addr"]), Hint: str(out["hint"]), Close: func() error {
		_, err := d.run(context.Background(), "debugger.detach", protocol.Args{"service": s.Name})
		return err
	}}, nil
}

type logsBridge struct{ base }

func (l *logsBridge) Logs(ctx context.Context, q core.LogQuery) (<-chan core.LogLine, error) {
	return l.stream(ctx, "logs.query", protocol.Args{"services": q.Services, "instance": q.Instance, "since": q.Since.String(),
		"tail": q.Tail, "follow": q.Follow, "match": q.Match, "regex": q.Regex})
}

// ---- data ----

type databaseBridge struct{ base }

func (d *databaseBridge) Query(ctx context.Context, db, q string) (core.Table, error) {
	return d.table(ctx, "database.query", protocol.Args{"database": db, "query": q})
}

func (d *databaseBridge) Exec(ctx context.Context, db, q string) (int64, error) {
	out, err := d.run(ctx, "database.exec", protocol.Args{"database": db, "query": q})
	return int64(number(out["rows"])), err
}

func (d *databaseBridge) Databases(ctx context.Context) ([]string, error) {
	rows, err := d.rows(ctx, "database.list", nil)
	var out []string
	for _, r := range rows {
		out = append(out, str(r["database"]))
	}
	return out, err
}

func (d *databaseBridge) CreateDatabase(ctx context.Context, name string) error {
	_, err := d.run(ctx, "database.create", protocol.Args{"name": name})
	return err
}

func (d *databaseBridge) DropDatabase(ctx context.Context, name string) error {
	_, err := d.run(ctx, "database.drop", protocol.Args{"name": name})
	return err
}

func (d *databaseBridge) QueryLanguage() string { return "sql" }
func (d *databaseBridge) RunQuery(ctx context.Context, q string) (core.Table, error) {
	return d.Query(ctx, "", q)
}

type cacheBridge struct{ base }

func (c *cacheBridge) Do(ctx context.Context, args ...string) (string, error) {
	out, err := c.run(ctx, "cache.do", protocol.Args{"args": args})
	return str(out["reply"]), err
}

func (c *cacheBridge) Info(ctx context.Context) (map[string]string, error) {
	rows, err := c.rows(ctx, "cache.info", nil)
	out := map[string]string{}
	for _, r := range rows {
		out[str(r["key"])] = str(r["value"])
	}
	return out, err
}

type queueBridge struct{ base }

func (q *queueBridge) Queues(ctx context.Context) ([]core.Queue, error) {
	rows, err := q.rows(ctx, "queue.list", nil)
	var out []core.Queue
	for _, r := range rows {
		out = append(out, core.Queue{Name: str(r["name"]), Messages: integer(r["messages"]), Ready: integer(r["ready"]), Unacked: integer(r["unacked"]),
			Consumers: integer(r["consumers"]), InRate: number(r["in_rate"]), OutRate: number(r["out_rate"])})
	}
	return out, err
}

func (q *queueBridge) Purge(ctx context.Context, queue string) error {
	_, err := q.run(ctx, "queue.purge", protocol.Args{"queue": queue})
	return err
}

func (q *queueBridge) Publish(ctx context.Context, target string, body []byte) error {
	_, err := q.run(ctx, "queue.publish", protocol.Args{"target": target, "body": string(body)})
	return err
}

type kvBridge struct{ base }

func (k *kvBridge) Get(ctx context.Context, key string) ([]byte, bool, error) {
	rows, err := k.rows(ctx, "kv.get", protocol.Args{"key": key})
	if err != nil || len(rows) == 0 {
		return nil, false, err
	}
	return []byte(str(rows[0]["value"])), rows[0]["found"] != false, nil
}

func (k *kvBridge) Put(ctx context.Context, key string, value []byte) error {
	_, err := k.run(ctx, "kv.put", protocol.Args{"key": key, "value": string(value)})
	return err
}

func (k *kvBridge) Delete(ctx context.Context, key string) error {
	_, err := k.run(ctx, "kv.delete", protocol.Args{"key": key})
	return err
}

func (k *kvBridge) List(ctx context.Context, prefix string) ([]string, error) {
	rows, err := k.rows(ctx, "kv.list", protocol.Args{"prefix": prefix})
	var out []string
	for _, r := range rows {
		out = append(out, str(r["key"]))
	}
	return out, err
}

type loadBridge struct{ base }

func (l *loadBridge) Start(ctx context.Context) error {
	_, err := l.run(ctx, "load.start", nil)
	return err
}
func (l *loadBridge) Stop(ctx context.Context) error {
	_, err := l.run(ctx, "load.stop", nil)
	return err
}
func (l *loadBridge) SetRate(ctx context.Context, rps float64) error {
	_, err := l.run(ctx, "load.rate", protocol.Args{"rps": rps})
	return err
}

func (l *loadBridge) Status(ctx context.Context) (core.LoadStatus, error) {
	rows, err := l.rows(ctx, "load.status", nil)
	if err != nil || len(rows) == 0 {
		return core.LoadStatus{}, err
	}
	r := rows[0]
	st := core.LoadStatus{Running: r["running"] == true, Rate: number(r["rate"]), Sent: int64(number(r["sent"])), Failed: int64(number(r["failed"])),
		Latency: core.Latency{P50: duration(r["p50"]), P95: duration(r["p95"]), P99: duration(r["p99"])}}
	if m, ok := r["extra"].(map[string]any); ok {
		st.Extra = map[string]string{}
		for k, v := range m {
			st.Extra[k] = str(v)
		}
	}
	return st, nil
}

// ---- machines ----

type hostsBridge struct{ base }

func (h *hostsBridge) Hosts(ctx context.Context) ([]core.Host, error) {
	rows, err := h.rows(ctx, "hosts.list", nil)
	var out []core.Host
	for _, r := range rows {
		out = append(out, core.Host{Name: str(r["name"]), Addr: str(r["addr"]), Roles: strs(r["roles"]), Ready: r["ready"] == true, Reason: str(r["reason"]),
			OS: str(r["os"]), CPUs: integer(r["cpus"]), CPUUsed: number(r["cpu_used"]), MemTotal: int64(number(r["mem_total"])),
			MemUsed: int64(number(r["mem_used"])), Load1: number(r["load1"])})
	}
	return out, err
}

func (h *hostsBridge) Shell(ctx context.Context, host string, cmd []string) (*exec.Cmd, error) {
	out, err := h.run(ctx, "hosts.shell", protocol.Args{"host": host, "command": cmd})
	if err != nil {
		return nil, err
	}
	return command(ctx, out)
}

// ---- values off the wire: JSON numbers, RFC 3339 or unix-millisecond times, Go or millisecond durations ----

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	}
	return fmt.Sprint(v)
}

func number(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

func integer(v any) int { return int(number(v)) }

func when(v any) time.Time {
	if s, ok := v.(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
	}
	if n := number(v); n != 0 {
		return time.UnixMilli(int64(n))
	}
	return time.Time{}
}

func duration(v any) time.Duration {
	if s, ok := v.(string); ok {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	return time.Duration(number(v) * float64(time.Millisecond))
}

func strs(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, str(x))
		}
		return out
	}
	return nil
}
