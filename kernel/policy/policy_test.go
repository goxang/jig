package policy

import (
	"testing"

	"github.com/goxang/jig/kernel/identity"
)

var envs = map[string]Env{"loadtest2": {Protected: true}, "prod": {Readonly: true}}

func TestLocalDefault(t *testing.T) {
	me := identity.Subject{Kind: identity.KindUser, Name: "me", Roles: []string{"owner"}}
	var c *Config
	for _, tc := range []struct {
		s      identity.Subject
		effect string
		env    string
		want   Decision
	}{
		{me, "read", "prod", Allow},
		{me, "write", "local", Allow},
		{me, "write", "prod", Deny},
		{me, "write", "loadtest2", Confirm},
		{me, "danger", "local", Confirm},
		{identity.AI(me), "write", "local", Approve},
		{identity.AI(me), "read", "local", Allow},
		{identity.Agent("fixer", []string{"x.*"}, me), "write", "local", Approve},
		{identity.Agent("fixer", nil, me), "write", "local", Allow},
		{identity.Agent("fixer", nil, me), "write", "loadtest2", Approve},
	} {
		if d, _ := c.Decide(Request{Subject: tc.s, Action: "x.y", Effect: tc.effect, Env: tc.env}, envs); d != tc.want {
			t.Errorf("%s %s@%s = %s, want %s", tc.s, tc.effect, tc.env, d, tc.want)
		}
	}
}

func TestRoles(t *testing.T) {
	c := &Config{
		Roles: map[string]Role{
			"viewer":   {Allow: []string{"*:read"}},
			"dev":      {Extends: "viewer", Allow: []string{"*:write@env/local"}},
			"operator": {Extends: "dev", Confirm: []string{"*:write@env/loadtest2"}, Reauth: []string{"*:danger"}, Allow: []string{"*:write@env/loadtest2"}},
			"ai":       {Allow: []string{"*:read"}, Approve: []string{"*:write"}},
		},
		Bind: map[string]string{"group:backend": "dev", "user:alice": "operator"},
	}
	alice := identity.Subject{Kind: identity.KindUser, Name: "alice"}
	bob := identity.Subject{Kind: identity.KindUser, Name: "bob", Groups: []string{"backend"}}
	for _, tc := range []struct {
		s      identity.Subject
		effect string
		env    string
		want   Decision
	}{
		{bob, "read", "loadtest2", Allow},
		{bob, "write", "local", Allow},
		{bob, "write", "loadtest2", Deny},
		{alice, "write", "loadtest2", Confirm},
		{alice, "danger", "local", Reauth},
		{alice, "write", "prod", Deny},
		{identity.AI(bob), "write", "local", Approve},
	} {
		if d, why := c.Decide(Request{Subject: tc.s, Action: "runtime.restart", Effect: tc.effect, Env: tc.env}, envs); d != tc.want {
			t.Errorf("%s %s@%s = %s (%s), want %s", tc.s, tc.effect, tc.env, d, why, tc.want)
		}
	}
}
