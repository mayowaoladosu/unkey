package cache

import (
	"github.com/unkeyed/unkey/pkg/cache/metrics"
)

const (
	defaultRevalidationQueueSize = 1000
	defaultRevalidationWorkers   = 10
)

// enqueueRevalidation schedules background refresh work for key. Duplicate
// refreshes for the same key are dropped before enqueue. When the worker queue
// is full the refresh is dropped instead of blocking the caller.
func (c *cache[K, V]) enqueueRevalidation(key K, work func()) {
	c.inflightMu.Lock()
	if c.inflightRefreshes[key] {
		c.inflightMu.Unlock()
		return
	}
	c.inflightRefreshes[key] = true
	c.inflightMu.Unlock()

	select {
	case c.revalidateC <- func() {
		defer func() {
			c.inflightMu.Lock()
			delete(c.inflightRefreshes, key)
			c.inflightMu.Unlock()
		}()
		work()
	}:
	default:
		metrics.CacheRevalidationsDropped.WithLabelValues(c.resource).Inc()
		c.inflightMu.Lock()
		delete(c.inflightRefreshes, key)
		c.inflightMu.Unlock()
	}
}

func (c *cache[K, V]) enqueueRevalidationBatch(work func()) {
	select {
	case c.revalidateC <- work:
	default:
		metrics.CacheRevalidationsDropped.WithLabelValues(c.resource).Inc()
	}
}
