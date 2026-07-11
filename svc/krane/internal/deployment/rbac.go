package deployment

import (
	"context"
	"encoding/json"
	"fmt"

	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/ptr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

// ensureDeploymentServiceAccount creates a ServiceAccount for the deployment.
// The SA is referenced by podSpec.ServiceAccountName so the pod doesn't use the
// namespace default. automountServiceAccountToken is false — no API access is needed.
// The SA is owned by the ReplicaSet via ownerRef for automatic GC.
func (c *Controller) ensureDeploymentServiceAccount(ctx context.Context, req *ctrlv1.ApplyDeployment) error {
	saName := workloadResourcePrefix(req)
	commonLabels := deploymentLabels(req)

	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: req.GetK8SNamespace(),
			Labels:    commonLabels,
		},
		AutomountServiceAccountToken: ptr.P(false),
	}
	if err := serverSideApplyResource(ctx, c.clientSet.CoreV1().RESTClient(), "serviceaccounts", req.GetK8SNamespace(), saName, sa); err != nil {
		return fmt.Errorf("failed to apply service account: %w", err)
	}

	return nil
}

// patchOwnerRef patches the ownerReferences on the Secret and ServiceAccount
// for the given resource name so they are garbage-collected with the ReplicaSet.
func (c *Controller) patchOwnerRef(ctx context.Context, namespace, name string, ownerRef metav1.OwnerReference) error {
	ownerPatch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"ownerReferences": []metav1.OwnerReference{ownerRef},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal owner ref patch: %w", err)
	}

	_, err = c.clientSet.CoreV1().Secrets(namespace).Patch(
		ctx, name, types.MergePatchType, ownerPatch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch secret owner ref: %w", err)
	}

	_, err = c.clientSet.CoreV1().ServiceAccounts(namespace).Patch(
		ctx, name, types.MergePatchType, ownerPatch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch service account owner ref: %w", err)
	}

	return nil
}

func serverSideApplyResource(ctx context.Context, restClient rest.Interface, resource, namespace, name string, obj any) error {
	patch, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = restClient.Patch(types.ApplyPatchType).
		Namespace(namespace).
		Resource(resource).
		Name(name).
		Param("fieldManager", fieldManagerKrane).
		Body(patch).
		Do(ctx).
		Get()
	return err
}
