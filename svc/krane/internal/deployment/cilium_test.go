package deployment

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/krane/pkg/labels"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildCiliumPolicyUsesExplicitResourceGrants(t *testing.T) {
	req := fullApplyRequest(t)
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: req.GetK8SName(), UID: "rs-uid"}}

	policy := buildCiliumNetworkPolicy(req, replicaSetOwnerRef(rs))
	endpoint, found, err := unstructured.NestedStringMap(policy.Object, "spec", "endpointSelector", "matchLabels")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, req.GetDeploymentId(), endpoint[labels.LabelKeyDeploymentID])
	require.Equal(t, req.GetResourceId(), endpoint[labels.LabelKeyResourceID])

	ingress, found, err := unstructured.NestedSlice(policy.Object, "spec", "ingress")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, ingress, 2, "public Frontline and one private caller grant")
	egress, found, err := unstructured.NestedSlice(policy.Object, "spec", "egress")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, egress, 1, "one declared private binding grant")
	require.Equal(t, map[string]interface{}{
		"toEndpoints": []interface{}{map[string]interface{}{
			"matchLabels": map[string]interface{}{labels.LabelKeyResourceID: "resource_api"},
		}},
		"toPorts": []interface{}{map[string]interface{}{
			"ports": []interface{}{map[string]interface{}{"port": "8081", "protocol": "TCP"}},
		}},
	}, egress[0])

	req.Public = false
	policy = buildCiliumNetworkPolicy(req, replicaSetOwnerRef(rs))
	ingress, found, err = unstructured.NestedSlice(policy.Object, "spec", "ingress")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, ingress, 1, "private service keeps only explicit callers")

	req.AllowedCallers = nil
	req.Bindings = nil
	policy = buildCiliumNetworkPolicy(req, replicaSetOwnerRef(rs))
	ingress, found, err = unstructured.NestedSlice(policy.Object, "spec", "ingress")
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, ingress, "unbound private service is default-deny")
	egress, found, err = unstructured.NestedSlice(policy.Object, "spec", "egress")
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, egress, "resource without bindings gets no private egress")
}
