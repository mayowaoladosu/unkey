package router

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/cache"
	"github.com/unkeyed/unkey/pkg/clock"
	"github.com/unkeyed/unkey/svc/frontline/internal/db"
)

func TestApplyRouteVersionClearsCacheOnlyAfterAChange(t *testing.T) {
	routes, err := cache.New(cache.Config[string, db.FindFrontlineRouteByFQDNRow]{
		Fresh:    time.Minute,
		Stale:    time.Minute,
		MaxSize:  10,
		Resource: "test_frontline_routes",
		Clock:    clock.New(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	routes.Set(ctx, "app.example.test", db.FindFrontlineRouteByFQDNRow{DeploymentID: "d_old"})
	service := &service{frontlineRouteCache: routes}
	state := routeUpdateState{}

	service.applyRouteVersion(ctx, &state, 10)
	_, hit := routes.Get(ctx, "app.example.test")
	require.Equal(t, cache.Hit, hit, "initial observation must not flush a warm cache")

	service.applyRouteVersion(ctx, &state, 10)
	_, hit = routes.Get(ctx, "app.example.test")
	require.Equal(t, cache.Hit, hit)

	service.applyRouteVersion(ctx, &state, 11)
	_, hit = routes.Get(ctx, "app.example.test")
	require.Equal(t, cache.Miss, hit)
}
