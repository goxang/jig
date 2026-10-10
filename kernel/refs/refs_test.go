package refs

import "testing"

func TestParseRoundTrip(t *testing.T) {
	for _, s := range []string{"svc://api", "db://db/dbo.transactions?id=7", "file://cmd/main.go#L12", "k8s://ns/deployment/api"} {
		r, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if r.String() != s {
			t.Errorf("%s round-trips to %s", s, r)
		}
	}
	if _, err := Parse("api"); err == nil {
		t.Error("parsed a ref without a scheme")
	}
}

func TestExpand(t *testing.T) {
	if got := Expand("svc://{service}/{port}/{x}", map[string]any{"service": "api", "port": 80}); got != "svc://api/80/{x}" {
		t.Errorf("got %s", got)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	if err := r.Claim("jira", "jira"); err != nil {
		t.Fatal(err)
	}
	if err := r.Claim("svc", "x"); err == nil {
		t.Error("a plugin took a core scheme")
	}
	if o, _ := r.Owner("JIRA"); o != "jira" {
		t.Errorf("owner %q", o)
	}
}
