package cache

import "sync"

type fetchCall struct {
	wg         sync.WaitGroup
	result     any
	err        error
	panicValue any
}

// fetchDeduped runs fn once per key while concurrent callers wait and share
// the result. Panics in fn are propagated to every waiter.
func (c *cache[K, V]) fetchDeduped(key K, fn func() (V, error)) (V, error) {
	value, err := c.fetchDedupedAny(key, func() (any, error) {
		return fn()
	})

	typed, ok := value.(V)
	if !ok {
		var zero V
		return zero, err
	}
	return typed, err
}

func (c *cache[K, V]) fetchDedupedWithKey(key K, fn func() (V, K, error)) (V, K, error) {
	type paired struct {
		value V
		key   K
	}

	value, err := c.fetchDedupedAny(key, func() (any, error) {
		v, cacheKey, fetchErr := fn()
		return paired{value: v, key: cacheKey}, fetchErr
	})

	var zeroV V
	var zeroK K
	if err != nil {
		return zeroV, zeroK, err
	}

	typed, ok := value.(paired)
	if !ok {
		return zeroV, zeroK, err
	}
	return typed.value, typed.key, err
}

func (c *cache[K, V]) fetchDedupedAny(key K, fn func() (any, error)) (any, error) {
	c.fetchMu.Lock()
	if call, ok := c.fetchInflight[key]; ok {
		c.fetchMu.Unlock()
		call.wg.Wait()
		if call.panicValue != nil {
			panic(call.panicValue)
		}
		return call.result, call.err
	}

	call := &fetchCall{ //nolint:exhaustruct // wg.Add runs before any waiter can observe zero state
	}
	call.wg.Add(1)
	c.fetchInflight[key] = call
	c.fetchMu.Unlock()

	func() {
		defer func() {
			if r := recover(); r != nil {
				call.panicValue = r
			}
			c.fetchMu.Lock()
			delete(c.fetchInflight, key)
			c.fetchMu.Unlock()
			call.wg.Done()
		}()
		call.result, call.err = fn()
	}()

	if call.panicValue != nil {
		panic(call.panicValue)
	}
	return call.result, call.err
}
