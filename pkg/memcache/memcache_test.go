package memcache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cassianobraz/payment-processor/pkg/memcache"
)

func TestSetGet(t *testing.T) {
	c := memcache.New[string](time.Minute, time.Minute)
	defer c.Close()

	c.Set("k", "v")

	got, ok := c.Get("k")
	if !ok || got != "v" {
		t.Fatalf("Get(k) = %q, %v; want %q, true", got, ok, "v")
	}
}

func TestExpiration(t *testing.T) {
	// TTL far below the sweep interval: the entry expires before the janitor runs,
	// so this exercises the lazy expiry check in Get.
	c := memcache.New[int](10*time.Millisecond, time.Hour)
	defer c.Close()

	c.Set("k", 42)
	time.Sleep(30 * time.Millisecond)

	if got, ok := c.Get("k"); ok {
		t.Fatalf("Get(k) = %d, true; want expired", got)
	}
}

func TestIncrementIsAtomic(t *testing.T) {
	c := memcache.New[int](time.Minute, time.Minute)
	defer c.Close()

	add := func(current int, exists bool) int {
		if !exists {
			return 1
		}
		return current + 1
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { c.Increment("card:123", add) })
	}
	wg.Wait()

	if got, ok := c.Get("card:123"); !ok || got != 100 {
		t.Fatalf("Get(card:123) = %d, %v; want 100, true", got, ok)
	}
}

func TestConcurrentAccessAcrossShards(t *testing.T) {
	c := memcache.New[int](time.Minute, time.Minute)
	defer c.Close()

	var wg sync.WaitGroup
	for i := range 200 {
		wg.Go(func() {
			key := fmt.Sprintf("key-%d", i%50)
			c.Set(key, i)
			c.Get(key)
		})
	}
	wg.Wait()
}

func TestJanitorEvictsExpired(t *testing.T) {
	c := memcache.New[int](10*time.Millisecond, 5*time.Millisecond)
	defer c.Close()

	const n = 1000
	for i := range n {
		c.Set(fmt.Sprintf("key-%d", i), i)
	}

	// Len counts entries still stored, expired or not, so it only reaches zero
	// after the janitor has swept every shard. Poll instead of sleeping a fixed time.
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	deadline := time.After(2 * time.Second)

	for c.Len() != 0 {
		select {
		case <-poll.C:
		case <-deadline:
			t.Fatalf("Len() = %d after deadline; want 0", c.Len())
		}
	}
}
