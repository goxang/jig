package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/goxang/jig/protocol"
)

func TestTerminalRunsAProgram(t *testing.T) {
	term, err := newTerminal(protocol.Node{Widget: "terminal", Props: protocol.Args{"command": []any{"sh"}}}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := term.start(40, 6); err != nil {
		t.Fatal(err)
	}
	defer term.close()
	term.input = true
	for _, r := range "echo hi-$((40+2))" {
		term.key(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	term.key(tea.KeyPressMsg{Code: tea.KeyEnter})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(term.lines(), "\n"), "hi-42") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	lines := term.lines()
	t.Logf("%q", lines)
	if !strings.Contains(strings.Join(lines, "\n"), "hi-42") || len(lines) != 6 {
		t.Fatalf("screen: %q", lines)
	}
}
