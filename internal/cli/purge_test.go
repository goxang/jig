package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPurgeKeepsPinnedRecentAndNewestBackup(t *testing.T) {
	root := t.TempDir()
	old := time.Now().AddDate(0, 0, -40)
	write := func(rel string, aged bool) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if aged {
			os.Chtimes(p, old, old)
		}
		return p
	}
	stale := write("projects/a/sessions/s1/state.json", true)
	pinned := write("projects/a/sessions/s2/state.json", true)
	pin := write("projects/a/sessions/pinned", true)
	os.WriteFile(pin, []byte("s2\n"), 0o644)
	os.Chtimes(pin, old, old)
	fresh := write("projects/a/reports/r1", false)
	secret := write("projects/a/secrets", true)
	gone := write("projects/b/ai/x", true)
	b1 := write("env/consul-backup-1.json", true)
	b2 := write("env/consul-backup-2.json", true)
	os.Chtimes(b2, old.Add(time.Hour), old.Add(time.Hour))
	kv1 := write("env/kv-backup/one", true)
	kv2 := write("env/kv-backup/two", false)
	audit := write("env/audit.jsonl", true)

	p := &purger{cutoff: time.Now().AddDate(0, 0, -30)}
	p.projects(filepath.Join(root, "projects"))
	p.backups(filepath.Join(root, "env"), false)

	for _, f := range []string{pinned, pin, fresh, secret, b2, kv2, audit} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("%s should stay: %v", f, err)
		}
	}
	for _, f := range []string{stale, gone, b1, kv1, filepath.Join(root, "projects/b")} {
		if _, err := os.Stat(f); err == nil {
			t.Errorf("%s should go", f)
		}
	}
}
