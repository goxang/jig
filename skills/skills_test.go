package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The built-in AI, given the jig-plugin skill, writes a plugin that passes jig plugin test. It runs
// the AI set up with jig ai config, so it is opt-in: JIG_AI_CHECK=1 go test -run Sentry ./skills
func TestAIWritesASentryPlugin(t *testing.T) {
	if os.Getenv("JIG_AI_CHECK") == "" {
		t.Skip("JIG_AI_CHECK unset")
	}
	bin := filepath.Join(t.TempDir(), "jig")
	if out, err := exec.Command("go", "build", "-o", bin, "../cmd/jig").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	project := "version: 1\nproject: sentrycheck\ndefault: dev\nenvironments:\n  dev: {runtime: {type: local}}\n"
	if err := os.WriteFile(filepath.Join(dir, "jig.yaml"), []byte(project), 0o600); err != nil {
		t.Fatal(err)
	}
	jig := func(in string, args ...string) {
		t.Helper()
		c := exec.Command(bin, args...)
		c.Dir, c.Env = in, append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := c.CombinedOutput()
		t.Logf("jig %v:\n%s", args, out)
		if err != nil {
			t.Fatalf("jig %v: %v", args, err)
		}
	}
	jig(dir, "ai", "write a plugin that shows Sentry issues as a tab")
	found, _ := filepath.Glob(filepath.Join(dir, "*", "jig-plugin.yaml"))
	if len(found) != 1 {
		t.Fatalf("want one plugin folder, found %v", found)
	}
	jig(filepath.Dir(found[0]), "plugin", "test")
}
