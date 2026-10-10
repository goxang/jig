package contracts

import (
	"testing"

	"github.com/goxang/jig/kernel/view"
	"github.com/goxang/jig/protocol"
)

func TestContractsLoadAndCheck(t *testing.T) {
	if len(IDs()) != 23 || Get("metrics").Kind != "metrics" || ForKind("messaging").ID != "queue/1" {
		t.Fatalf("contracts: %v", IDs())
	}
	for _, id := range IDs() {
		if c := Get(id); c.View != nil {
			var v view.View
			if err := v.Load(c.View); err != nil {
				t.Fatalf("%s view: %v", id, err)
			}
		}
	}
	c := Get("queue/1")
	p := &protocol.Provides{
		Sources: map[string]protocol.SourceSpec{"queue.list": {}, "queue.info": {}},
		Actions: map[string]protocol.ActionSpec{"queue.purge": {Effect: "danger"}, "queue.publish": {Effect: "read"}},
	}
	got := c.Check(p)
	if len(got) != 2 || got[0].Name != "queue.publish" || got[1].Name != "inspect" {
		t.Fatalf("problems: %v", got)
	}
	f := &protocol.Frame{Fields: []protocol.Field{{Name: "name", Type: "string"}, {Name: "messages", Type: "string"}}}
	if ps := fieldProblems("queue.list", map[string]string{"name": "string", "messages": "int", "ready": "int"}, f); len(ps) != 2 {
		t.Fatalf("field problems: %v", ps)
	}
}
