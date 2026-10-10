package tui

import (
	"testing"
)

func TestCommandMenuAndNearest(t *testing.T) {
	if m := cmdMenu("/mo"); len(m) != 1 || m[0].name != "/model" {
		t.Fatalf("menu = %v", m)
	}
	if m := cmdMenu("/model x"); m != nil {
		t.Fatalf("a command with words after it has no menu: %v", m)
	}
	if n := nearestCommand("/sesions"); n != "/sessions" {
		t.Fatalf("nearest = %q", n)
	}
	if n := nearestCommand("/deploy-everything"); n != "" {
		t.Fatalf("nothing is near, got %q", n)
	}
}
