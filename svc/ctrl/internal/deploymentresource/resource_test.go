package deploymentresource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStableResourceIdentities(t *testing.T) {
	webID, err := ID("d_test", "web")
	require.NoError(t, err)
	require.Regexp(t, `^resource_[a-f0-9]{32}$`, webID)
	webAgain, err := ID("d_test", "web")
	require.NoError(t, err)
	require.Equal(t, webID, webAgain)

	primary, err := K8sName("deploy-test", "web", true)
	require.NoError(t, err)
	require.Equal(t, "deploy-test", primary)
	worker, err := K8sName("deploy-test", "email_worker", false)
	require.NoError(t, err)
	require.Regexp(t, `^deploy-test-email-worker-[a-f0-9]{8}$`, worker)
	require.LessOrEqual(t, len(worker), 63)

	long, err := K8sName("deployment-with-a-name-that-is-already-close-to-the-kubernetes-limit", "worker_with_a_very_long_name", false)
	require.NoError(t, err)
	require.LessOrEqual(t, len(long), 63)
}
