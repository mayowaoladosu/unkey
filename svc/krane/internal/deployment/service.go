package deployment

import (
	"context"
	"encoding/json"
	"fmt"

	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ensureWorkloadService creates a stable ClusterIP endpoint for HTTP services
// and functions. Public traffic may still go directly through Frontline, while
// private bindings resolve this service inside the workspace namespace.
func (c *Controller) ensureWorkloadService(ctx context.Context, req *ctrlv1.ApplyDeployment, owner metav1.OwnerReference) error {
	service := buildWorkloadService(req, owner)
	patch, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal workload service: %w", err)
	}
	_, err = c.clientSet.CoreV1().Services(req.GetK8SNamespace()).Patch(
		ctx,
		req.GetK8SName(),
		types.ApplyPatchType,
		patch,
		metav1.PatchOptions{FieldManager: fieldManagerKrane},
	)
	if err != nil {
		return fmt.Errorf("failed to apply workload service: %w", err)
	}
	return nil
}

func buildWorkloadService(req *ctrlv1.ApplyDeployment, owner metav1.OwnerReference) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            req.GetK8SName(),
			Namespace:       req.GetK8SNamespace(),
			Labels:          deploymentLabels(req),
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: workloadSelector(req),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       req.GetPort(),
				TargetPort: intstr.FromInt32(req.GetPort()),
			}},
		},
	}
}
