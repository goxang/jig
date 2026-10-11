package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDesktopApps(t *testing.T) {
	home, sys := t.TempDir(), t.TempDir()
	write := func(dir, name, body string) {
		d := filepath.Join(dir, "applications")
		_ = os.MkdirAll(d, 0o755)
		if err := os.WriteFile(filepath.Join(d, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(sys, "office.desktop", "[Desktop Entry]\nName=Office\nMimeType=application/pdf;text/plain;\n[Desktop Action new]\nName=New\n")
	write(sys, "hidden.desktop", "[Desktop Entry]\nName=Hidden\nNoDisplay=true\nMimeType=text/plain;\n")
	write(sys, "mine.desktop", "[Desktop Entry]\nName=System\nMimeType=text/plain;\n")
	write(home, "mine.desktop", "[Desktop Entry]\nName=Mine\nMimeType=text/markdown;\n")
	t.Setenv("XDG_DATA_HOME", home)
	t.Setenv("XDG_DATA_DIRS", sys)

	apps := desktopApps()
	if len(apps) != 2 || apps[0].name != "Mine" || apps[1].name != "Office" {
		t.Fatalf("apps = %+v", apps)
	}
	if !slices.Equal(apps[1].mimes, []string{"application/pdf", "text/plain"}) || apps[0].id != "mine.desktop" {
		t.Fatalf("apps = %+v", apps)
	}
}
