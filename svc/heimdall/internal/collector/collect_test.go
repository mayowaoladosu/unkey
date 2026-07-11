package collector

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildPodInfoCarriesDeploymentResourceDimensions(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-pod",
			Labels: map[string]string{
				LabelComponent:              "deployment",
				LabelWorkspace:              "ws_test",
				LabelProject:                "project_test",
				LabelEnv:                    "env_test",
				LabelDeployment:             "deployment_test",
				LabelDeploymentResource:     "resource_web",
				LabelDeploymentResourceKind: "service",
			},
			Annotations: map[string]string{AnnotationDeploymentResourceName: "web"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "deployment"}}},
	}

	info := buildPodInfo(pod)
	require.Equal(t, "deployment_test", info.resourceID, "billing identity remains the parent deployment")
	require.Equal(t, "resource_web", info.deploymentResourceID)
	require.Equal(t, "web", info.deploymentResourceName)
	require.Equal(t, "service", info.deploymentResourceKind)
}
