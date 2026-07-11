package router

import (
	"context"
	"fmt"
	"time"

	"github.com/unkeyed/unkey/pkg/logger"
)

type routeUpdateState struct {
	initialized bool
	version     int64
}

// WatchRouteChanges polls one trigger-maintained revision row instead of every
// hostname. When it changes, this Frontline process drops its hostname cache so
// the next request resolves the new deployment synchronously.
func (s *service) WatchRouteChanges(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("route cache poll interval must be positive")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	state := routeUpdateState{}

	for {
		if err := s.refreshRouteVersion(ctx, &state); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Warn("unable to poll frontline route updates", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *service) refreshRouteVersion(ctx context.Context, state *routeUpdateState) error {
	version, err := s.db.FindFrontlineRouteRevision(ctx)
	if err != nil {
		return err
	}
	s.applyRouteVersion(ctx, state, version)
	return nil
}

func (s *service) applyRouteVersion(ctx context.Context, state *routeUpdateState, version int64) {
	if !state.initialized {
		state.initialized = true
		state.version = version
		return
	}
	if state.version == version {
		return
	}

	previous := state.version
	state.version = version
	s.frontlineRouteCache.Clear(ctx)
	logger.Info("invalidated frontline route cache", "previousVersion", previous, "version", version)
}
