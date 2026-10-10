package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOneFetchPerKey(t *testing.T) {
	c := New()
	var calls atomic.Int32
	fetch := func() (any, error) { calls.Add(1); time.Sleep(20 * time.Millisecond); return 7, nil }
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if v, _ := c.Get("k", time.Minute, fetch); v != 7 {
				t.Error(v)
			}
		})
	}
	wg.Wait()
	c.Get("k", time.Minute, fetch)
	if calls.Load() != 1 {
		t.Fatalf("fetched %d times", calls.Load())
	}
	c.Forget("k")
	c.Get("k", time.Minute, fetch)
	if calls.Load() != 2 {
		t.Fatal("forget kept the value")
	}
}
