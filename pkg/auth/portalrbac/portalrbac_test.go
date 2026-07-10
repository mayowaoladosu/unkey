package portalrbac_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/unkeyed/unkey/pkg/auth/portalrbac"
	"github.com/unkeyed/unkey/pkg/rbac"
)

func TestParseRejectsUnknownCapability(t *testing.T) {
	_, err := portalrbac.Parse("keys:destroy")
	require.Error(t, err)

	c, err := portalrbac.Parse("keys:reroll")
	require.NoError(t, err)
	require.Equal(t, portalrbac.CapKeysReroll, c)
}

func TestCapabilitiesUseExactMatching(t *testing.T) {
	granted := []string{
		string(portalrbac.CapKeysRead),
		string(portalrbac.CapKeysCreate),
		string(portalrbac.CapAnalyticsRead),
	}

	require.NoError(t, rbac.Check(rbac.S(string(portalrbac.CapKeysRead)), granted))
	require.NoError(t, rbac.Check(rbac.S(string(portalrbac.CapKeysCreate)), granted))
	require.NoError(t, rbac.Check(rbac.S(string(portalrbac.CapAnalyticsRead)), granted))
	require.Error(t, rbac.Check(rbac.S(string(portalrbac.CapKeysReroll)), granted),
		"keys:create must not imply keys:reroll")
}
