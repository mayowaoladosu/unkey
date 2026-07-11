package deploy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

	for _, output := range manifest.Outputs {
		resourceID, err := deploymentresource.ID(deployment.ID, output.Name)
		if err != nil {
			return nil, err
		}
		kind, err := resourceKind(output.Kind)
		if err != nil {
			return nil, err
		}
		public := output.Public || output.Name == publicOutput
		command := output.Command
		if len(command) == 0 && output.Kind == deploymanifest.OutputKindContainer {
			command = manifest.Runtime.Command
		}
		port := output.Port
		if output.Kind == deploymanifest.OutputKindFunction && port == 0 {
			port = 8080
		}

		k8sName := sql.NullString{}
		image := sql.NullString{}
		if output.Kind != deploymanifest.OutputKindStatic {
			name, nameErr := deploymentresource.K8sName(deployment.K8sName, output.Name, primaryRuntime)
			if nameErr != nil {
				return nil, nameErr
			}
			primaryRuntime = false
			k8sName = sql.NullString{Valid: true, String: name}
			image = deployment.Image
			if !image.Valid || image.String == "" {
				return nil, fmt.Errorf("runtime resource %q requires a deployment image", output.Name)
			}
		}
		encodedCommand, err := json.Marshal(command)
		if err != nil {
			return nil, fmt.Errorf("encode resource command %q: %w", output.Name, err)
		}
		protocol := db.DeploymentResourcesUpstreamProtocolHttp1
		if output.UpstreamProtocol == string(db.DeploymentResourcesUpstreamProtocolH2c) {
			protocol = db.DeploymentResourcesUpstreamProtocolH2c
		}

		resources = append(resources, db.InsertDeploymentResourceParams{
			ID:               resourceID,
			DeploymentID:     deployment.ID,
			WorkspaceID:      deployment.WorkspaceID,
			ProjectID:        deployment.ProjectID,
			AppID:            deployment.AppID,
			EnvironmentID:    deployment.EnvironmentID,
			Name:             output.Name,
			Kind:             kind,
			K8sName:          k8sName,
			Image:            image,
			Command:          encodedCommand,
			Port:             port,
			UpstreamProtocol: protocol,
			Public:           public,
			Schedule:         nullableString(output.Schedule),
			Runtime:          nullableString(output.Runtime),
			Handler:          nullableString(output.Handler),
			CpuMillicores:    deployment.CpuMillicores,
			MemoryMib:        deployment.MemoryMib,
			StorageMib:       deployment.StorageMib,
			CreatedAt:        now,
		})
	}

	err := restate.RunVoid(ctx, func(runCtx restate.RunContext) error {
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

func inferredPublicOutput(outputs []deploymanifest.Output) string {
	for _, output := range outputs {
		if output.Public {
			return output.Name
		}
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
