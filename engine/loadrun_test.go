package engine

import (
	"testing"
	"time"
)

func TestProfileRate(t *testing.T) {
	m := time.Minute
	for _, c := range []struct {
		profile        string
		elapsed, total time.Duration
		want           float64
	}{
		{"steady", 0, m, 100}, {"soak", 30 * time.Second, 0, 100},
		{"ramp", 0, 10 * m, 1}, {"ramp", 4 * m, 10 * m, 50}, {"ramp", 9 * m, 10 * m, 100}, {"ramp", 30 * time.Second, 0, 50},
		{"spike", m, 10 * m, 20}, {"spike", 5 * m, 10 * m, 100}, {"spike", 7 * m, 10 * m, 20},
	} {
		if got := ProfileRate(c.profile, 100, c.elapsed, c.total); got != c.want {
			t.Errorf("%s at %s of %s = %v, want %v", c.profile, c.elapsed, c.total, got, c.want)
		}
	}
}

func TestAsserts(t *testing.T) {
	r := &LoadResult{Sent: 1000, Failed: 3, Throughput: 1600, P95: 150 * time.Millisecond}
	for s, want := range map[string]bool{
		"p95 < 200ms": true, "p95 < 100ms": false, "errors < 0.5%": true, "errors < 0.1%": false,
		"throughput >= 1500/s": true, "failed <= 2": false,
	} {
		as, err := parseAssert(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := as.check(r); got.OK != want {
			t.Errorf("%s: %+v, want ok %v", s, got, want)
		}
	}
	for _, bad := range []string{"p95 200ms", "p95 ~ 1s", "lat < 1s", "p95 < fast"} {
		if _, err := parseAssert(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}
