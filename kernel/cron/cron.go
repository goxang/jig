// Package cron reads 5-field cron schedules.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Due reports whether the 5-field cron spec (minute hour day-of-month month day-of-week) fires in
// t's minute. Both day fields restricted means either matches, as in cron.
func Due(cron string, t time.Time) (bool, error) {
	if cron == "" {
		return false, nil
	}
	f := strings.Fields(cron)
	if len(f) != 5 {
		return false, fmt.Errorf("schedule %q: want 5 fields: minute hour day month weekday", cron)
	}
	limits := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}
	vals := [5]int{t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())}
	var hit [5]bool
	for i, field := range f {
		ok, err := matchField(field, vals[i], limits[i][0], limits[i][1])
		if err != nil {
			return false, fmt.Errorf("schedule %q: %w", cron, err)
		}
		hit[i] = ok
	}
	day := hit[2] && hit[4]
	if f[2] != "*" && f[4] != "*" {
		day = hit[2] || hit[4]
	}
	return hit[0] && hit[1] && hit[3] && day, nil
}

func matchField(field string, v, lo, hi int) (bool, error) {
	for _, part := range strings.Split(field, ",") {
		rng, step, hasStep := strings.Cut(part, "/")
		n := 1
		if hasStep {
			var err error
			if n, err = strconv.Atoi(step); err != nil || n < 1 {
				return false, fmt.Errorf("bad step %q", part)
			}
		}
		from, to := lo, hi
		if rng != "*" {
			a, b, isRange := strings.Cut(rng, "-")
			var err1, err2 error
			from, err1 = strconv.Atoi(a)
			to = from
			if isRange {
				to, err2 = strconv.Atoi(b)
			} else if hasStep {
				to = hi
			}
			if err1 != nil || err2 != nil || from < lo || to > hi || from > to {
				return false, fmt.Errorf("bad field %q", part)
			}
		}
		if v >= from && v <= to && (v-from)%n == 0 {
			return true, nil
		}
	}
	return false, nil
}
