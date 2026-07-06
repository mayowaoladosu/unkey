package router

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/unkeyed/unkey/pkg/cache"
	"github.com/unkeyed/unkey/pkg/clock"
	"github.com/unkeyed/unkey/svc/frontline/internal/caches"
	"github.com/unkeyed/unkey/svc/frontline/internal/db"
)

func TestGetPolicies_CachesParseErrorWithoutFailingOpen(t *testing.T) {
	t.Parallel()

	policyCache, err := cache.New(cache.Config[string, caches.CachedPolicies]{
		Fresh:    30 * time.Second,
		Stale:    5 * time.Minute,
		MaxSize:  64_000_000,
		Resource: "policies_test",
		Clock:    clock.New(),
	})
	require.NoError(t, err)

	s := &service{
		policyCache: policyCache,
	}

	route := db.FindFrontlineRouteByFQDNRow{
		DeploymentID:   "dep_broken",
		SentinelConfig: []byte(`not-valid-sentinel-config`),
	}

	ctx := context.Background()

	_, err = s.getPolicies(ctx, route)
	require.Error(t, err)

	_, hit := policyCache.Get(ctx, route.DeploymentID)
	require.Equal(t, cache.Hit, hit)

	_, err = s.getPolicies(ctx, route)
	require.Error(t, err)
}
