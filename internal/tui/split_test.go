package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSplitDragIsKept(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg)
	t.Setenv("HOME", t.TempDir())
	m := &model{}
	if got := m.paneSize("kv", splitGeo{total: 100, minA: 20, minB: 30}, 40, 0, 0, 10); got != 40 {
		t.Fatalf("default size %d", got)
	}
	if m.borderAt(39, 5) != "kv" {
		t.Fatal("no border at the left pane's right edge")
	}
	m.splitting = "kv"
	m.splitDrag(mouseEvent{Mouse: tea.Mouse{X: 59}, Action: mouseRelease})
	if got := m.paneSize("kv", splitGeo{total: 100, minA: 20, minB: 30}, 40, 0, 0, 10); got != 60 {
		t.Fatalf("dragged size %d", got)
	}
	if got := loadSplits()["kv"]; got != 0.6 {
		t.Fatalf("saved %v", got)
	}
	m.splitting = "kv"
	m.splitDrag(mouseEvent{Mouse: tea.Mouse{X: 99}, Action: mouseMotion})
	if got := m.paneSize("kv", splitGeo{total: 100, minA: 20, minB: 30}, 40, 0, 0, 10); got != 70 {
		t.Fatalf("clamped size %d", got)
	}
}
