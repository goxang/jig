package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFollowReadsHowATaskEnded(t *testing.T) {
	base := filepath.Join(t.TempDir(), "1")
	write := func(ext, s string) {
		if err := os.WriteFile(base+ext, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".log", "step one\nstep two\n")
	write(".done", "0")
	j := &job{Log: base, PID: os.Getpid()}
	if err := follow(j); err != nil {
		t.Fatal(err)
	}
	if out := strings.Join(j.Output(), "|"); out != "step one|step two" {
		t.Fatalf("output %q", out)
	}

	write(".done", "1")
	write(".err", "task deploy: step 2 failed")
	if err := follow(&job{Log: base, PID: os.Getpid()}); err == nil || err.Error() != "task deploy: step 2 failed" {
		t.Fatalf("err %v", err)
	}

	os.Remove(base + ".done")
	if err := follow(&job{Log: base, PID: -1}); err == nil || !strings.Contains(err.Error(), "killed") {
		t.Fatalf("a task gone without a done file: %v", err)
	}
}
