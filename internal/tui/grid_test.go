package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPrintable(t *testing.T) {
	if got := printable("a\tb\r\nc\x07\x1b[1md"); got != "a b  c\x1b[1md" {
		t.Fatalf("%q", got)
	}
}

func TestNameFilter(t *testing.T) {
	cases := []struct {
		p, s string
		want bool
	}{
		{"", "x", true}, {"sett", "parser.settlement.Request", true}, {"*settle*", "parser.settlement.Request", true},
		{"parser*", "dispatcher.parser", false}, {"*.request.*", "parser.settlement.Request.*.*.*", true},
	}
	for _, c := range cases {
		if got := rowMatcher(c.p, []string{"name"})([]string{c.s}); got != c.want {
			t.Errorf("%q ~ %q = %v", c.p, c.s, got)
		}
	}
}

func TestHighlightKeepsCellColours(t *testing.T) {
	open, _, _ := strings.Cut(sHover.Render("\x00"), "\x00")
	got := highlight(sHover, sGreen.Render("ok")+" x", 6)
	if !strings.HasPrefix(got, open) || !strings.Contains(got, "\x1b[m"+open+" x  ") {
		t.Fatalf("%q", got)
	}
	if stripANSI(got) != "ok x" {
		t.Fatalf("text changed: %q", stripANSI(got))
	}
}

func TestGridAltArrowsOrder(t *testing.T) {
	g := newGrid("t", gcol{title: "n"})
	g.key(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModAlt})
	if !g.desc {
		t.Error("alt+down did not sort descending")
	}
	g.key(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModAlt})
	if g.desc {
		t.Error("alt+up did not sort ascending")
	}
}

func TestGridCtrlAltArrowsSort(t *testing.T) {
	g := newGrid("t", gcol{title: "a"}, gcol{title: "b"})
	g.sortBy = 0
	if !g.key(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl | tea.ModAlt}) || g.sortBy != 1 {
		t.Errorf("ctrl+alt+right: sortBy %d", g.sortBy)
	}
	g.key(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl | tea.ModAlt})
	if !g.desc {
		t.Error("ctrl+alt+down did not sort descending")
	}
}
