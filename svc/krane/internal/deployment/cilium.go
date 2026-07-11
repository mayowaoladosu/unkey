package deployment

import (
	"context"
	"fmt"
	"strconv"

	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/svc/krane/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ensureCiliumNetworkPolicy creates or updates a CiliumNetworkPolicy that
// grants public Frontline ingress, explicit private callers, and only the
// private binding egress declared by this resource. Cilium's default-deny
// applies in each direction selected by the policy.
//
// The policy is namespaced to the deployment's namespace and owned by its
// ReplicaSet or CronJob so it is garbage-collected with the workload.
func (c *Controller) ensureCiliumNetworkPolicy(
	ctx context.Context,
	req *ctrlv1.ApplyDeployment,
	owner metav1.OwnerReference,
) error {
	policy := buildCiliumNetworkPolicy(req, owner)
	policyName := policy.GetName()

	gvr := schema.GroupVersionResource{
		Group:    "cilium.io",
		Version:  "v2",
		Resource: "ciliumnetworkpolicies",
	}

	// Server-side apply so concurrent reconciles converge instead of
	// fighting over field ownership.
	_, err := c.dynamicClient.Resource(gvr).Namespace(req.GetK8SNamespace()).Apply(
		ctx,
		policyName,
		policy,
		metav1.ApplyOptions{FieldManager: fieldManagerKrane},
	)
	if err != nil {
		return fmt.Errorf("failed to apply cilium network policy: %w", err)
	}

	return nil
}

func buildCiliumNetworkPolicy(req *ctrlv1.ApplyDeployment, owner metav1.OwnerReference) *unstructured.Unstructured {
	policyName := fmt.Sprintf("%s-frontline-ingress", req.GetK8SName())
	endpointLabels := map[string]interface{}{
		labels.LabelKeyDeploymentID: req.GetDeploymentId(),
	}
	if req.GetResourceId() != "" {
		endpointLabels[labels.LabelKeyResourceID] = req.GetResourceId()
	}
	ingress := make([]interface{}, 0, len(req.GetAllowedCallers())+1)
	if req.GetPublic() {
		ingress = append(ingress, ciliumIngressRule(req.GetPort(), map[string]interface{}{
			labels.LabelKeyNamespace: frontlineNamespace,
		}))
	}
	for _, callerID := range req.GetAllowedCallers() {
		ingress = append(ingress, ciliumIngressRule(req.GetPort(), map[string]interface{}{
			labels.LabelKeyResourceID: callerID,
		}))
	}
	egress := make([]interface{}, 0, len(req.GetBindings()))
	for _, binding := range req.GetBindings() {
		egress = append(egress, ciliumEgressRule(binding.GetPort(), map[string]interface{}{
			labels.LabelKeyResourceID: binding.GetResourceId(),
		}))
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cilium.io/v2",
			"kind":       "CiliumNetworkPolicy",
			"metadata": map[string]interface{}{
				"name":      policyName,
				"namespace": req.GetK8SNamespace(),
				"labels": labels.New().
					WorkspaceID(req.GetWorkspaceId()).
					ProjectID(req.GetProjectId()).
					AppID(req.GetAppId()).
					EnvironmentID(req.GetEnvironmentId()).
					DeploymentID(req.GetDeploymentId()).
					ResourceID(req.GetResourceId()).
					ResourceKind(resourceKindName(req)).
					ManagedByKrane().
					ComponentCiliumNetworkPolicy(),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         owner.APIVersion,
						"kind":               owner.Kind,
						"name":               owner.Name,
						"uid":                string(owner.UID),
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"endpointSelector": map[string]interface{}{
					"matchLabels": endpointLabels,
				},
				"ingress": ingress,
				"egress":  egress,
			},
		},
	}
}

func ciliumIngressRule(port int32, fromLabels map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"fromEndpoints": []interface{}{
			map[string]interface{}{"matchLabels": fromLabels},
		},
		"toPorts": []interface{}{
			map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"port":     strconv.Itoa(int(port)),
						"protocol": "TCP",
					},
				},
			},
		},
	}
}

func ciliumEgressRule(port int32, toLabels map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"toEndpoints": []interface{}{
			map[string]interface{}{"matchLabels": toLabels},
		},
		"toPorts": []interface{}{
			map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"port":     strconv.Itoa(int(port)),
						"protocol": "TCP",
					},
				},
			},
		},
	}
}
