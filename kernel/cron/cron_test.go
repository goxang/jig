package cron

import (
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	at := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC) // a Monday
	for cron, want := range map[string]bool{
		"* * * * *":        true,
		"30 9 * * *":       true,
		"*/15 * * * *":     true,
		"*/7 * * * *":      false,
		"0 9 * * *":        false,
		"30 9 * * 1-5":     true,
		"30 9 * * 0,6":     false,
		"30 9 1 * 1":       true,
		"30 9 1 * 2":       false,
		"0,30 8-10 * 10 *": true,
	} {
		if got, err := Due(cron, at); err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", cron, got, err, want)
		}
	}
	for _, bad := range []string{"* * * *", "61 * * * *", "*/0 * * * *", "a * * * *"} {
		if _, err := Due(bad, at); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}
