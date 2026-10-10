package engine

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestReportExtras(t *testing.T) {
	was := &ReportResult{Stats: []string{"avg"}, Rows: []ReportRow{{Metric: "p95", Values: map[string]float64{"avg": 100}}}}
	now := &ReportResult{Name: "nightly", Stats: []string{"avg"}, Rows: []ReportRow{{Metric: "p95", Values: map[string]float64{"avg": 120}}},
		Load: &LoadResult{Name: "smoke", Sent: 10, Checks: []AssertResult{{Assert: "p95 < 100ms", Got: "120ms"}}}}
	now.Compare = compareRows(was, now)
	if len(now.Compare) != 1 || now.Compare[0].Change() != "+20.0%" {
		t.Fatalf("compare %+v", now.Compare)
	}
	if !now.Failed() {
		t.Error("a broken load assertion must fail the report")
	}
	var md, ju, cs bytes.Buffer
	now.Markdown(&md)
	_ = now.Write(&ju, "junit")
	_ = now.Write(&cs, "csv")
	for out, want := range map[string]string{md.String(): "+20.0%", ju.String(): `failures="1"`, cs.String(): "p95,,avg,120"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	text, err := (&App{}).renderTemplate(context.Background(), "# Night\n{{.Load}}", now)
	if err != nil || !strings.Contains(text, "# Night") || !strings.Contains(text, "✗ p95 < 100ms") {
		t.Fatalf("template: %v\n%s", err, text)
	}
}
