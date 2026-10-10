package host

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/blake2b"
)

// fakeRegistry publishes plugins as a mirror directory: index.json and <name>/<version>/<file>.
func fakeRegistry(t *testing.T, plugins map[string]map[string]string) string {
	t.Helper()
	mirror := t.TempDir()
	ix := Index{Plugins: map[string]*IndexEntry{}}
	for name, versions := range plugins {
		e := &IndexEntry{Publisher: "someone", Versions: map[string]*IndexVersion{}}
		for v, extra := range versions {
			src := t.TempDir()
			manifest := "name: " + name + "\nversion: " + v + "\n" + extra
			os.WriteFile(filepath.Join(src, ManifestFile), []byte(manifest), 0o644)
			data, err := tarDir(src)
			if err != nil {
				t.Fatal(err)
			}
			os.MkdirAll(filepath.Join(mirror, name, v), 0o755)
			os.WriteFile(filepath.Join(mirror, name, v, "p.tar.gz"), data, 0o644)
			sum := sha256.Sum256(data)
			m, _ := ParseManifest([]byte(manifest), "x/"+ManifestFile)
			e.Versions[v] = &IndexVersion{Requires: m.Requires, Permissions: m.Permissions,
				Artifacts: map[string]Artifact{"any": {URL: "https://example.invalid/p.tar.gz", SHA256: hex.EncodeToString(sum[:])}}}
		}
		ix.Plugins[name] = e
	}
	b, _ := json.Marshal(ix)
	os.WriteFile(filepath.Join(mirror, "index.json"), b, 0o644)
	return mirror
}

func isolate(t *testing.T) {
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(k, t.TempDir())
	}
}

func TestInstallPackVendorGC(t *testing.T) {
	isolate(t)
	ctx := context.Background()
	mirror := fakeRegistry(t, map[string]map[string]string{
		"git":      {"1.0.0": "", "1.1.0": "", "1.2.0": "", "2.0.0": ""},
		"pack-std": {"1.0.0": "requires: { git: ^1.1 }\ntasks: { hi: [echo hi] }\n"},
		"net":      {"1.0.0": "permissions: { net: [example.com] }\n"},
	})
	proj := t.TempDir()
	in := &Installer{Dir: proj, Registry: &Registry{Mirror: mirror}, Yes: true}
	if err := in.Add(ctx, []string{"pack/std"}); err != nil {
		t.Fatal(err)
	}
	lock, _ := ReadLock(proj)
	if g := lock.Plugins["git"]; g == nil || g.Version != "1.1.0" || g.Pack != "pack-std" {
		t.Fatalf("git locked as %+v, want 1.1.0 from the pack (minimal version selection)", g)
	}
	ms, warn := Discover(proj, nil)
	if len(ms) != 2 || len(warn) != 0 {
		t.Fatalf("discovered %d plugins, warnings %v", len(ms), warn)
	}

	if err := in.Up(ctx, []string{"git"}, false); err != nil {
		t.Fatal(err)
	}
	if lock, _ = ReadLock(proj); lock.Plugins["git"].Version != "1.2.0" {
		t.Fatalf("up went to %s, want 1.2.0 (same major)", lock.Plugins["git"].Version)
	}

	asked := ""
	strict := &Installer{Dir: proj, Registry: &Registry{Mirror: mirror}, Ask: func(q string) bool { asked = q; return false }}
	if err := strict.Add(ctx, []string{"net"}); err == nil || !strings.Contains(asked, "permissions") {
		t.Fatalf("new permissions went unasked: %v %q", err, asked)
	}

	if _, err := in.Vendor(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir()) // a clean machine
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	offline := &Installer{Dir: proj, Registry: &Registry{Mirror: filepath.Join(mirror, "gone")}}
	if _, err := offline.Ensure(ctx); err == nil || !strings.Contains(err.Error(), "jig trust") {
		t.Fatalf("an untrusted, non-interactive install went ahead: %v", err)
	}
	offline.Yes = true
	if _, err := offline.Ensure(ctx); err != nil {
		t.Fatalf("offline install from vendor: %v", err)
	}
	if ms, _ := Discover(proj, nil); len(ms) != 2 {
		t.Fatalf("after the vendor install: %d plugins", len(ms))
	}

	if err := in.Remove([]string{"pack/std"}); err != nil {
		t.Fatal(err)
	}
	if lock, _ = ReadLock(proj); len(lock.Plugins) != 0 {
		t.Fatalf("pack removal left %v", lock.Names())
	}
	removed, err := GC()
	if err != nil || len(removed) != 2 {
		t.Fatalf("gc removed %v, %v", removed, err)
	}
}

func TestChecksumMismatchRefused(t *testing.T) {
	isolate(t)
	mirror := fakeRegistry(t, map[string]map[string]string{"x": {"1.0.0": ""}})
	os.WriteFile(filepath.Join(mirror, "x/1.0.0/p.tar.gz"), []byte("tampered"), 0o644)
	in := &Installer{Dir: t.TempDir(), Registry: &Registry{Mirror: mirror}, Yes: true}
	if err := in.Add(context.Background(), []string{"x"}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered artifact: %v", err)
	}
}

func TestResolve(t *testing.T) {
	v := func(req map[string]string) *IndexVersion { return &IndexVersion{Requires: req} }
	ix := &Index{Plugins: map[string]*IndexEntry{
		"a": {Versions: map[string]*IndexVersion{"1.0.0": v(map[string]string{"c": "^1.1"})}},
		"b": {Versions: map[string]*IndexVersion{"1.0.0": v(map[string]string{"c": "1.3"})}},
		"c": {Versions: map[string]*IndexVersion{"1.1.0": v(nil), "1.3.0": v(nil), "1.4.0": v(nil), "2.0.0": v(nil)}},
		"d": {Versions: map[string]*IndexVersion{"1.0.0": v(map[string]string{"c": "^2"})}},
	}}
	got, err := ix.Resolve(map[string]string{"a": "1.0.0", "b": "1.0.0"})
	if err != nil || got["c"] != "1.3.0" {
		t.Fatalf("c = %s, %v; want 1.3.0, the highest minimum", got["c"], err)
	}
	if _, err := ix.Resolve(map[string]string{"a": "1.0.0", "d": "1.0.0"}); err == nil {
		t.Fatal("two majors of c resolved")
	}
}

func TestParseSource(t *testing.T) {
	for in, want := range map[string]string{
		"gitlab":                          "registry gitlab ",
		"pack/product@1.2.0":              "registry pack-product 1.2.0",
		"github.com/o/r/sub@v1.2.0":       "git https://github.com/o/r sub v1.2.0",
		"git+ssh://git@host/grp/r.git@v1": "git ssh://git@host/grp/r.git  v1",
		"./tools/jig-foo":                 "path ./tools/jig-foo",
		"https://x/p.tar.gz#sha256=abc":   "tarball https://x/p.tar.gz abc",
	} {
		s := parseSource(in)
		var got string
		switch s.kind {
		case "registry":
			got = strings.Join([]string{s.kind, s.name, s.version}, " ")
		case "git":
			got = strings.Join([]string{s.kind, s.repo, s.sub, s.ref}, " ")
		case "path":
			got = s.kind + " " + s.path
		case "tarball":
			got = strings.Join([]string{s.kind, s.url, s.sha}, " ")
		}
		if got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

func TestMinisign(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	id := []byte("12345678")
	pk := base64.StdEncoding.EncodeToString(append(append([]byte("Ed"), id...), pub...))
	data := []byte("artifact bytes")
	h := blake2b.Sum512(data)
	sig := ed25519.Sign(priv, h[:])
	trusted := "timestamp:1 file:p.tar.gz"
	global := ed25519.Sign(priv, append(append([]byte{}, sig...), trusted...))
	text := "untrusted comment: x\n" + base64.StdEncoding.EncodeToString(append(append([]byte("ED"), id...), sig...)) +
		"\ntrusted comment: " + trusted + "\n" + base64.StdEncoding.EncodeToString(global) + "\n"
	if err := verifyMinisign(pk, text, data); err != nil {
		t.Fatal(err)
	}
	if err := verifyMinisign(pk, text, []byte("other bytes")); err == nil {
		t.Fatal("a signature over other bytes verified")
	}
}
