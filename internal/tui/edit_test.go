package tui

import (
	"testing"

	"charm.land/bubbles/v2/textarea"

	tea "charm.land/bubbletea/v2"
)

func TestEditKeySelection(t *testing.T) {
	in := newInput()
	in.Focus()
	in.SetValue("select name from users")
	var s textSel
	press := func(k tea.KeyPressMsg) {
		if !editKey(&in, &s, k) {
			in, _ = in.Update(k)
		}
	}
	press(keyOf("ctrl+shift+left"))
	press(keyOf("orders"))
	if got := in.Value(); got != "select name from orders" {
		t.Fatalf("typing over a selected word: %q", got)
	}
	press(keyOf("ctrl+h"))
	if got := in.Value(); got != "select name from " {
		t.Fatalf("ctrl+backspace: %q", got)
	}
	press(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := in.Value(); got != "select name from" {
		t.Fatalf("backspace: %q", got)
	}
	press(keyOf("ctrl+a"))
	press(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if in.Value() != "" {
		t.Fatalf("ctrl+a then backspace: %q", in.Value())
	}
}

func TestChatInputWalksWords(t *testing.T) {
	in := textarea.New()
	editorTextarea(&in)
	in.Focus()
	in.SetValue("one two three")
	in, _ = in.Update(keyOf("ctrl+left"))
	in.InsertString("X")
	if got := in.Value(); got != "one two Xthree" {
		t.Errorf("ctrl+left then typing = %q", got)
	}
}

func TestChatInputSelectionAndWordDelete(t *testing.T) {
	in := textarea.New()
	editorTextarea(&in)
	in.Focus()
	in.SetValue("one two three")
	in, _ = in.Update(keyOf("ctrl+backspace"))
	if got := in.Value(); got != "one two " {
		t.Errorf("ctrl+backspace = %q", got)
	}
	in.SelectAll()
	in, _ = in.Update(tea.PasteMsg{Content: "new"})
	if got := in.Value(); got != "new" {
		t.Errorf("paste over a selection = %q", got)
	}
}
