package deploy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymanifest"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymentresource"
)

func (w *Workflow) materializeDeploymentResources(
	ctx restate.ObjectContext,
	deployment db.Deployment,
	manifest deploymanifest.Manifest,
) ([]db.DeploymentResource, error) {
	publicOutput := inferredPublicOutput(manifest.Outputs)
	resources := make([]db.InsertDeploymentResourceParams, 0, len(manifest.Outputs))
	now := time.Now().UnixMilli()
	primaryRuntime := true
	identities := make(map[string]materializedResourceIdentity, len(manifest.Outputs))

	for _, output := range manifest.Outputs {
		resourceID, err := deploymentresource.ID(deployment.ID, output.Name)
		if err != nil {
			return nil, err
		}
		port := output.Port
		if output.Kind == deploymanifest.OutputKindFunction && port == 0 {
			port = 8080
		}

		k8sName := sql.NullString{}
		if output.Kind != deploymanifest.OutputKindStatic {
			name, nameErr := deploymentresource.K8sName(deployment.K8sName, output.Name, primaryRuntime)
			if nameErr != nil {
				return nil, nameErr
			}
			primaryRuntime = false
			k8sName = sql.NullString{Valid: true, String: name}
			if !deployment.Image.Valid || deployment.Image.String == "" {
				return nil, fmt.Errorf("runtime resource %q requires a deployment image", output.Name)
			}
		}
		identities[output.Name] = materializedResourceIdentity{
			id:      resourceID,
			k8sName: k8sName,
			port:    port,
		}
	}

	bindingsByOutput, allowedCallers, err := resolvePrivateBindings(manifest.Outputs, identities)
	if err != nil {
		return nil, err
	}

	for _, output := range manifest.Outputs {
		identity := identities[output.Name]
		kind, err := resourceKind(output.Kind)
		if err != nil {
			return nil, err
		}
		public := output.Public || output.Name == publicOutput
		command := output.Command
		if len(command) == 0 && output.Kind == deploymanifest.OutputKindContainer {
			command = manifest.Runtime.Command
		}
		if len(command) == 0 && output.Kind == deploymanifest.OutputKindFunction {
			command, err = functionRuntimeCommand(output.Runtime)
			if err != nil {
				return nil, err
			}
		}
		image := deployment.Image
		if output.Kind == deploymanifest.OutputKindStatic {
			image = sql.NullString{}
		}
		encodedCommand, err := json.Marshal(command)
		if err != nil {
			return nil, fmt.Errorf("encode resource command %q: %w", output.Name, err)
		}
		protocol := db.DeploymentResourcesUpstreamProtocolHttp1
		if output.UpstreamProtocol == string(db.DeploymentResourcesUpstreamProtocolH2c) {
			protocol = db.DeploymentResourcesUpstreamProtocolH2c
		}
		encodedBindings, err := json.Marshal(bindingsByOutput[output.Name])
		if err != nil {
			return nil, fmt.Errorf("encode bindings for resource %q: %w", output.Name, err)
		}
		callers := allowedCallers[identity.id]
		if callers == nil {
			callers = []string{}
		}
		encodedAllowedCallers, err := json.Marshal(callers)
		if err != nil {
			return nil, fmt.Errorf("encode allowed callers for resource %q: %w", output.Name, err)
		}

		resources = append(resources, db.InsertDeploymentResourceParams{
			ID:               identity.id,
			DeploymentID:     deployment.ID,
			WorkspaceID:      deployment.WorkspaceID,
			ProjectID:        deployment.ProjectID,
			AppID:            deployment.AppID,
			EnvironmentID:    deployment.EnvironmentID,
			Name:             output.Name,
			Kind:             kind,
			K8sName:          identity.k8sName,
			Image:            image,
			Command:          encodedCommand,
			Port:             identity.port,
			UpstreamProtocol: protocol,
			Public:           public,
			Schedule:         nullableString(output.Schedule),
			Runtime:          nullableString(output.Runtime),
			Handler:          nullableString(output.Handler),
			Bindings:         encodedBindings,
			AllowedCallers:   encodedAllowedCallers,
			CpuMillicores:    deployment.CpuMillicores,
			MemoryMib:        deployment.MemoryMib,
			StorageMib:       deployment.StorageMib,
			CreatedAt:        now,
		})
	}

	err = restate.RunVoid(ctx, func(runCtx restate.RunContext) error {
		return db.Tx(runCtx, w.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
			queries := db.NewQueries(tx)
			for _, resource := range resources {
				if err := queries.InsertDeploymentResource(txCtx, resource); err != nil {
					return err
				}
			}
			return nil
		})
	}, restate.WithName("materialize deployment resources"), restate.WithMaxRetryAttempts(runMaxAttempts))
	if err != nil {
		return nil, fmt.Errorf("materialize deployment resources: %w", err)
	}

	return restate.Run(ctx, func(runCtx restate.RunContext) ([]db.DeploymentResource, error) {
		return w.db.ListDeploymentResourcesByDeployment(runCtx, deployment.ID)
	}, restate.WithName("list materialized deployment resources"), restate.WithMaxRetryAttempts(runMaxAttempts))
}

type materializedResourceIdentity struct {
	id      string
	k8sName sql.NullString
	port    int32
}

type resolvedPrivateBinding struct {
	Name         string                         `json:"name"`
	ResourceID   string                         `json:"resourceId"`
	ResourceName string                         `json:"resourceName"`
	Protocol     deploymanifest.BindingProtocol `json:"protocol"`
	Host         string                         `json:"host"`
	Port         int32                          `json:"port"`
}

func resolvePrivateBindings(
	outputs []deploymanifest.Output,
	identities map[string]materializedResourceIdentity,
) (map[string][]resolvedPrivateBinding, map[string][]string, error) {
	bindingsByOutput := make(map[string][]resolvedPrivateBinding, len(outputs))
	allowedCallers := make(map[string][]string, len(outputs))
	for _, output := range outputs {
		consumer := identities[output.Name]
		bindingsByOutput[output.Name] = make([]resolvedPrivateBinding, 0, len(output.Bindings))
		for _, binding := range output.Bindings {
			target, ok := identities[binding.Resource]
			if !ok || !target.k8sName.Valid || target.port < 1 {
				return nil, nil, fmt.Errorf("binding %q on resource %q has no routable target", binding.Name, output.Name)
			}
			protocol := binding.Protocol
			if protocol == "" {
				protocol = deploymanifest.BindingProtocolHTTP
			}
			bindingsByOutput[output.Name] = append(bindingsByOutput[output.Name], resolvedPrivateBinding{
				Name:         binding.Name,
				ResourceID:   target.id,
				ResourceName: binding.Resource,
				Protocol:     protocol,
				Host:         target.k8sName.String,
				Port:         target.port,
			})
			allowedCallers[target.id] = append(allowedCallers[target.id], consumer.id)
		}
	}
	for targetID := range allowedCallers {
		slices.Sort(allowedCallers[targetID])
	}
	return bindingsByOutput, allowedCallers, nil
}

func inferredPublicOutput(outputs []deploymanifest.Output) string {
	for _, output := range outputs {
		if output.Public {
			return output.Name
		}
	}
	if len(outputs) != 1 {
		return ""
	}
	for _, output := range outputs {
		switch output.Kind {
		case deploymanifest.OutputKindContainer, deploymanifest.OutputKindFunction, deploymanifest.OutputKindStatic:
			return output.Name
		default:
		}
	}
	return ""
}

func resourceKind(kind deploymanifest.OutputKind) (db.DeploymentResourcesKind, error) {
	switch kind {
	case deploymanifest.OutputKindContainer:
		return db.DeploymentResourcesKindService, nil
	case deploymanifest.OutputKindFunction:
		return db.DeploymentResourcesKindFunction, nil
	case deploymanifest.OutputKindWorker:
		return db.DeploymentResourcesKindWorker, nil
	case deploymanifest.OutputKindCron:
		return db.DeploymentResourcesKindCron, nil
	case deploymanifest.OutputKindStatic:
		return db.DeploymentResourcesKindStatic, nil
	default:
		return "", fmt.Errorf("unsupported deployment output kind %q", kind)
	}
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{Valid: value != "", String: value}
}
