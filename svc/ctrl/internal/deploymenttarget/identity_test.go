package deploymenttarget

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStableTargetAndAssignmentIDs(t *testing.T) {
	identity := Identity{
		AppID:         "app_test",
		EnvironmentID: "env_test",
		Kind:          KindBranch,
		Key:           "main",
	}

	first, err := ID(identity)
	require.NoError(t, err)
	second, err := ID(identity)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Regexp(t, `^target_[a-f0-9]{32}$`, first)

	other, err := ID(Identity{
		AppID:         identity.AppID,
		EnvironmentID: identity.EnvironmentID,
		Kind:          KindEnvironment,
		Key:           identity.Key,
	})
	require.NoError(t, err)
	require.NotEqual(t, first, other)

	assignment, err := AssignmentID(first, "invocation_1")
	require.NoError(t, err)
	require.Regexp(t, `^assignment_[a-f0-9]{32}$`, assignment)
	assignmentAgain, err := AssignmentID(first, "invocation_1")
	require.NoError(t, err)
	require.Equal(t, assignment, assignmentAgain)

	route, err := RouteID("storefront-production-acme.example.test")
	require.NoError(t, err)
	require.Regexp(t, `^route_[a-f0-9]{32}$`, route)
}

func TestStableIDsPreservePartBoundaries(t *testing.T) {
	left := stableID("target", "ab", "c")
	right := stableID("target", "a", "bc")
	require.NotEqual(t, left, right)
}
