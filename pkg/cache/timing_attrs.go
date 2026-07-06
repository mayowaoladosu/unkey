package cache

type timingAttrs struct {
	getMiss         map[string]string
	getFresh        map[string]string
	getStale        map[string]string
	swrFresh        map[string]string
	swrStale        map[string]string
	swrMiss         map[string]string
	swrFallbackFresh map[string]string
	swrFallbackStale map[string]string
	swrFallbackMiss  map[string]string
}

func newTimingAttrs(resource string) timingAttrs {
	return timingAttrs{
		getMiss:          map[string]string{"cache": resource, "status": "miss"},
		getFresh:         map[string]string{"cache": resource, "status": "fresh"},
		getStale:         map[string]string{"cache": resource, "status": "stale"},
		swrFresh:         map[string]string{"cache": resource, "status": "fresh"},
		swrStale:         map[string]string{"cache": resource, "status": "stale"},
		swrMiss:          map[string]string{"cache": resource, "status": "miss"},
		swrFallbackFresh: map[string]string{"cache": resource, "status": "fresh"},
		swrFallbackStale: map[string]string{"cache": resource, "status": "stale"},
		swrFallbackMiss:  map[string]string{"cache": resource, "status": "miss"},
	}
}
