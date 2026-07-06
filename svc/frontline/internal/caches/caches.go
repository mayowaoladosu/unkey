package caches

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/unkeyed/unkey/pkg/cache"
	"github.com/unkeyed/unkey/pkg/cache/middleware"
	"github.com/unkeyed/unkey/pkg/clock"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/frontline/internal/db"
)

// Caches holds all cache instances used throughout frontline.
type Caches struct {
	// HostName -> frontline Route
	FrontlineRoutes cache.Cache[string, db.FindFrontlineRouteByFQDNRow]

	// DeploymentID -> List of Instances
	InstancesByDeployment cache.Cache[string, []db.FindInstancesByDeploymentIDRow]

	// DeploymentID -> Parsed sentinel policies. Cached to avoid re-parsing
	// the protojson SentinelConfig on every request.
	Policies cache.Cache[string, CachedPolicies]

	// HostName -> Certificate
	TLSCertificates cache.Cache[string, tls.Certificate]
}

// Close shuts down the caches and cleans up resources.
func (c *Caches) Close() error {
	return nil
}

// Config defines the configuration options for initializing caches.
type Config struct {
	Clock clock.Clock

	// NodeID identifies this node (defaults to hostname-uniqueid to ensure uniqueness).
	NodeID string
}

func New(config Config) (*Caches, error) {
	if config.NodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		config.NodeID = fmt.Sprintf("%s-%s", hostname, uid.New("node"))
	}

	frontlineRoute, err := cache.New(cache.Config[string, db.FindFrontlineRouteByFQDNRow]{
		Fresh:    30 * time.Second,
		Stale:    5 * time.Minute,
		MaxSize:  routeCacheByteBudget,
		Cost:     frontlineRouteCost,
		Resource: "frontline_route",
		Clock:    config.Clock,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create frontline route cache: %w", err)
	}

	policies, err := cache.New(cache.Config[string, CachedPolicies]{
		Fresh:    30 * time.Second,
		Stale:    5 * time.Minute,
		MaxSize:  policiesCacheByteBudget,
		Cost:     cachedPoliciesCost,
		Resource: "policies",
		Clock:    config.Clock,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create policies cache: %w", err)
	}

	instancesByDeployment, err := cache.New(cache.Config[string, []db.FindInstancesByDeploymentIDRow]{
		Fresh:    10 * time.Second,
		Stale:    60 * time.Second,
		MaxSize:  10_000,
		Cost:     nil,
		Resource: "instances_by_deployment",
		Clock:    config.Clock,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create instances by deployment cache: %w", err)
	}

	tlsCertificate, err := cache.New(cache.Config[string, tls.Certificate]{
		Fresh:    time.Hour,
		Stale:    time.Hour * 12,
		MaxSize:  10_000,
		Cost:     nil,
		Resource: "tls_certificate",
		Clock:    config.Clock,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate cache: %w", err)
	}

	return &Caches{
		FrontlineRoutes:       middleware.WithTracing(frontlineRoute),
		InstancesByDeployment: middleware.WithTracing(instancesByDeployment),
		Policies:              middleware.WithTracing(policies),
		TLSCertificates:       middleware.WithTracing(tlsCertificate),
	}, nil
}
