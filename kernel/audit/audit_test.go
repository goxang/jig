package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendRedacts(t *testing.T) {
	p := filepath.Join(t.TempDir(), "local", "audit.jsonl")
	l := Open(p, "hunter22")
	for range 2 {
		if err := l.Append(Entry{Action: "db.query", Effect: "write", Decision: "allow", Args: map[string]any{"sql": "select 'hunter22'", "password": "x", "opts": map[string]any{"api_key": "y"}}}); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Count(s, "\n") != 2 || strings.Contains(s, "hunter22") || strings.Contains(s, `"x"`) || strings.Contains(s, `"y"`) {
		t.Errorf("log:\n%s", s)
	}
}
