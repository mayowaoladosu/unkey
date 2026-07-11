package deployment

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/krane/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildWorkloadServiceTargetsOneResource(t *testing.T) {
	req := fullApplyRequest(t)
	owner := metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: req.GetK8SName()}

	service := buildWorkloadService(req, owner)
	require.Equal(t, req.GetK8SName(), service.Name)
	require.Equal(t, req.GetK8SNamespace(), service.Namespace)
	require.Equal(t, map[string]string{
		labels.LabelKeyDeploymentID: req.GetDeploymentId(),
		labels.LabelKeyResourceID:   req.GetResourceId(),
	}, service.Spec.Selector)
	require.Len(t, service.Spec.Ports, 1)
	require.Equal(t, req.GetPort(), service.Spec.Ports[0].Port)
	require.Equal(t, req.GetPort(), service.Spec.Ports[0].TargetPort.IntVal)
	require.Equal(t, []metav1.OwnerReference{owner}, service.OwnerReferences)
}
