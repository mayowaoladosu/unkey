package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymenttarget"
)

func TestBuildDomainsSeparatesImmutableURLsFromMutableTargets(t *testing.T) {
	domains := buildDomains(
		"acme",
		"storefront",
		"web",
		"production",
		"0123456789abcdef",
		"feature/checkout",
		"",
		"example.test",
		false,
		"d_test",
	)

	require.Len(t, domains, 5)
	bySticky := make(map[db.FrontlineRoutesSticky]newDomain, len(domains))
	for _, domain := range domains {
		bySticky[domain.sticky] = domain
	}

	require.Empty(t, bySticky[db.FrontlineRoutesStickyNone].targetKind)
	require.Empty(t, bySticky[db.FrontlineRoutesStickyDeployment].targetKind)
	require.Equal(t, deploymenttarget.KindBranch, bySticky[db.FrontlineRoutesStickyBranch].targetKind)
	require.Equal(t, "feature/checkout", bySticky[db.FrontlineRoutesStickyBranch].targetKey)
	require.Equal(t, deploymenttarget.KindEnvironment, bySticky[db.FrontlineRoutesStickyEnvironment].targetKind)
	require.Equal(t, "production", bySticky[db.FrontlineRoutesStickyEnvironment].targetKey)
	require.Equal(t, deploymenttarget.KindLive, bySticky[db.FrontlineRoutesStickyLive].targetKind)
	require.Equal(t, "live", bySticky[db.FrontlineRoutesStickyLive].targetKey)
}
