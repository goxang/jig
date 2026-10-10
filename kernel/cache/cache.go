// Package cache keeps source answers for their TTL and runs one fetch per key at a time.
package cache

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry struct {
	v  any
	at time.Time
}

type Cache struct {
	mu sync.Mutex
	m  map[string]entry
	g  singleflight.Group
}

func New() *Cache { return &Cache{m: map[string]entry{}} }

// Get answers from the cache while key's value is younger than ttl, else fetches once for every
// caller waiting on key. Errors are not kept.
func (c *Cache) Get(key string, ttl time.Duration, fetch func() (any, error)) (any, error) {
	c.mu.Lock()
	e, ok := c.m[key]
	c.mu.Unlock()
	if ok && time.Since(e.at) < ttl {
		return e.v, nil
	}
	v, err, _ := c.g.Do(key, func() (any, error) {
		v, err := fetch()
		if err == nil && ttl > 0 {
			c.mu.Lock()
			c.m[key] = entry{v, time.Now()}
			c.mu.Unlock()
		}
		return v, err
	})
	return v, err
}

func (c *Cache) Forget(key string) {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
}
