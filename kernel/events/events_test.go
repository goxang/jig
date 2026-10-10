package events

import "testing"

func TestMatch(t *testing.T) {
	for _, c := range []struct {
		p, t string
		ok   bool
	}{{"deploy.*", "deploy.finished", true}, {"deploy.*", "deploy.x.y", false}, {"*", "a.b.c", true}, {"alert.firing", "alert.resolved", false}} {
		if Match(c.p, c.t) != c.ok {
			t.Errorf("Match(%q, %q) != %v", c.p, c.t, c.ok)
		}
	}
}

func TestHooks(t *testing.T) {
	b := NewBus()
	var ran []string
	stop := b.Hooks(map[string][]string{"deploy.*": {"notify"}}, func(steps []string, e Event) { ran = append(ran, steps[0]+" "+e.Topic) })
	b.Publish(Event{Topic: "deploy.finished"})
	b.Publish(Event{Topic: "alert.firing"})
	stop()
	b.Publish(Event{Topic: "deploy.started"})
	if len(ran) != 1 || ran[0] != "notify deploy.finished" {
		t.Errorf("ran %v", ran)
	}
}
