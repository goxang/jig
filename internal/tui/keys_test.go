package tui

import (
	"strings"

	"gopkg.in/yaml.v3"

	"testing"
	"time"

	"github.com/goxang/jig/contracts"

	"github.com/goxang/jig/engine"
	"github.com/goxang/jig/kernel/view"
	"github.com/goxang/jig/protocol"
	"github.com/goxang/jig/spec"
)

type namedTab struct {
	rootTab
	title string
}

func (n *namedTab) name() string { return n.title }

func TestKeyPresets(t *testing.T) {
	a, b := &namedTab{title: "Services"}, &namedTab{title: "Logs"}
	m := &model{tabs: []tab{a, b}, opened: map[int]bool{0: true, 1: true}, refreshed: map[int]time.Time{},
		app: &engine.App{Spec: &spec.Project{Keys: "jig"}}}
	if _, ok := m.globalKey("g"); !ok || m.prefix != "g" {
		t.Fatal("g is no prefix")
	}
	if m.globalKey("l"); m.active != 1 || m.prefix != "" {
		t.Fatalf("g l: active %d", m.active)
	}
	if m.globalKey(":"); m.prompt == nil {
		t.Fatal(": opened no palette")
	}
	m.prompt = nil
	m.command("open services")
	if m.active != 0 {
		t.Fatal(":open services")
	}
	if _, ok := m.globalKey("S"); ok {
		t.Fatal("S is the screen's under jig")
	}
	m.app.Spec.Keys = "rig"
	if m.keyFor("save") != "S" || m.keyFor("search") != "" {
		t.Fatal("rig preset")
	}
}

func TestBuiltinViewsLoad(t *testing.T) {
	var v view.View
	if err := v.Load(pluginsView); err != nil || v.Root.Widget != "table" || v.Root.Bind == nil || v.Root.Bind.Source != "plugins" {
		t.Fatalf("plugins view: %v %+v", err, v.Root)
	}
	a := &engine.App{Spec: &spec.Project{Dir: t.TempDir(), ViewOrder: []string{"a", "b"}, Views: map[string]*view.View{
		"a": {Title: "A", Slot: "home", Root: protocol.Node{Widget: "text"}},
		"b": {Title: "B", Slot: "tab", Root: protocol.Node{Widget: "text"}},
	}}}
	a.Spec.Plugins = map[string]*spec.Plugin{"x": {Off: true}}
	tabs := addViews(a, map[string]tab{})
	if len(tabs) != 3 || tabs[0].name() != "Home" || tabs[1].name() != "B" || tabs[2].name() != "Plugins" {
		t.Fatalf("tabs = %v", tabs)
	}
	var scm view.View
	if err := yaml.Unmarshal(contracts.Get("scm").View, &scm); err != nil || scm.Title != "Merge requests" {
		t.Fatal(err)
	}
	served(&scm.Root, "gl")
	var got []string
	view.Walk(&scm.Root, func(n *protocol.Node) {
		if n.Bind != nil {
			got = append(got, n.Bind.Source)
		}
		acts, _ := n.Props["actions"].([]any)
		for _, a := range acts {
			got = append(got, a.(map[string]any)["action"].(string))
		}
	})
	if strings.Join(got, " ") != "gl.scm.mrs gl.scm.comment gl.scm.pipelines" {
		t.Fatalf("served: %v", got)
	}
}
