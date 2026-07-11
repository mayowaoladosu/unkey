package cluster

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"

	"connectrpc.com/connect"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auth"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/pkg/metrics"
)

// changePageSize is the number of rows fetched per page when syncing deployment changes.
const changePageSize = 10000

// WatchDeploymentChanges streams incremental resource changes from the
// deployment_changes outbox table. When version_last_seen is 0, the server
// jumps to the current max pk and polls from there — it never replays
// historical changes.
func (s *Service) WatchDeploymentChanges(
	ctx context.Context,
	req *connect.Request[ctrlv1.WatchDeploymentChangesRequest],
	stream *connect.ServerStream[ctrlv1.DeploymentChangeEvent],
) error {
	if err := auth.Authenticate(req, s.bearer); err != nil {
		return err
	}

	region, err := s.resolveRegion(ctx, req.Msg.GetRegion())
	if err != nil {
		return err
	}

	versionCursor := req.Msg.GetVersionLastSeen()

	// When version is 0 and replay is not requested, jump to the current max pk
	// so we only see new changes.
	if versionCursor == 0 && !req.Msg.GetReplay() {
		maxVersion, err := s.db.GetDeploymentChangesMaxVersion(ctx, region.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		versionCursor = uint64(maxVersion)
		logger.Info("watch: starting from max version", "region_id", region.ID, "cursor", versionCursor)
	}

	// Poll deployment_changes for new entries.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		events, err := s.fetchDeploymentChangeEvents(ctx, region.ID, versionCursor)
		if err != nil {
			logger.Error("failed to fetch deployment change events", "error", err)
			return connect.NewError(connect.CodeInternal, err)
		}

		for _, event := range events {
			if err := stream.Send(event); err != nil {
				return err
			}
			if event.GetVersion() > versionCursor {
				versionCursor = event.GetVersion()
			}
		}

		if len(events) == 0 {
			jitter := time.Duration(500+rand.IntN(1000)) * time.Millisecond
			time.Sleep(jitter)
		}
	}
}

// fetchDeploymentChangeEvents polls deployment_changes for new entries and does a
// point lookup for each row to load current state.
func (s *Service) fetchDeploymentChangeEvents(ctx context.Context, regionID string, afterVersion uint64) ([]*ctrlv1.DeploymentChangeEvent, error) {
	changes, err := s.db.ListDeploymentChangesByRegionAll(ctx, db.ListDeploymentChangesByRegionAllParams{
		RegionID:     regionID,
		AfterVersion: afterVersion,
		Limit:        changePageSize,
	})
	if err != nil {
		return nil, err
	}

	events := make([]*ctrlv1.DeploymentChangeEvent, 0, len(changes))
	for _, change := range changes {
		resourceType := string(change.ResourceType)
		event, err := s.loadChangeEvent(ctx, change)
		if err != nil {
			if db.IsNotFound(err) {
				metrics.DeploymentChangesProcessedTotal.WithLabelValues(resourceType, "not_found").Inc()
			} else {
				metrics.DeploymentChangesProcessedTotal.WithLabelValues(resourceType, "error").Inc()
				logger.Error("failed to load state for deployment change",
					"error", err,
					"resource_type", change.ResourceType,
					"resource_id", change.ResourceID,
				)
			}
			// Skip this row but keep advancing the cursor
			events = append(events, &ctrlv1.DeploymentChangeEvent{Version: change.Pk})
			continue
		}
		metrics.DeploymentChangesProcessedTotal.WithLabelValues(resourceType, "success").Inc()
		if event != nil {
			events = append(events, event)
		}
	}

	return events, nil
}

// loadChangeEvent does a point lookup for a single deployment_changes row based on resource_type.
// Uses the control plane connection because deployment_changes rows arrive
// immediately after the data is written.
func (s *Service) loadChangeEvent(ctx context.Context, change db.DeploymentChange) (*ctrlv1.DeploymentChangeEvent, error) {
	switch change.ResourceType {
	case db.DeploymentChangesResourceTypeDeploymentTopology:
		if change.DeploymentResourceID != "" {
			row, err := s.db.FindDeploymentResourceTopology(ctx, db.FindDeploymentResourceTopologyParams{
				DeploymentID: change.ResourceID,
				ResourceID:   change.DeploymentResourceID,
				RegionID:     change.RegionID,
			})
			if err != nil {
				return nil, err
			}
			state, err := deploymentRowToState(deploymentRow{
				dt:              row.DeploymentTopology,
				d:               row.Deployment,
				resource:        &row.DeploymentResource,
				k8sNamespace:    row.K8sNamespace,
				environmentSlug: row.EnvironmentSlug,
				regionName:      row.RegionName,
				gitRepo:         row.GitRepo,
			}, change.Pk)
			if err != nil {
				return nil, err
			}
			return &ctrlv1.DeploymentChangeEvent{
				Version: change.Pk,
				Event:   &ctrlv1.DeploymentChangeEvent_Deployment{Deployment: state},
			}, nil
		}

		row, err := s.db.FindDeploymentTopologyByDeploymentAndRegion(ctx, db.FindDeploymentTopologyByDeploymentAndRegionParams{
			DeploymentID: change.ResourceID,
			RegionID:     change.RegionID,
		})
		if err != nil {
			return nil, err
		}
		state, err := deploymentRowToState(deploymentRow{
			dt:              row.DeploymentTopology,
			d:               row.Deployment,
			k8sNamespace:    row.K8sNamespace,
			environmentSlug: row.EnvironmentSlug,
			regionName:      row.RegionName,
			gitRepo:         row.GitRepo,
		}, change.Pk)
		if err != nil {
			return nil, err
		}
		if state == nil {
			return &ctrlv1.DeploymentChangeEvent{Version: change.Pk}, nil
		}
		return &ctrlv1.DeploymentChangeEvent{
			Version: change.Pk,
			Event:   &ctrlv1.DeploymentChangeEvent_Deployment{Deployment: state},
		}, nil

	case db.DeploymentChangesResourceTypeCiliumNetworkPolicy:
		// Cilium resources are no longer dispatched — frontline took
		// over the request path. The outbox row exists during the
		// cutover so we just acknowledge it and advance the version.
		return &ctrlv1.DeploymentChangeEvent{Version: change.Pk}, nil

	case db.DeploymentChangesResourceTypeSentinel:
		// Sentinel resources are no longer dispatched — frontline took
		// over the request path. The outbox row exists during the
		// cutover so we just acknowledge it and advance the version.
		return &ctrlv1.DeploymentChangeEvent{Version: change.Pk}, nil

	default:
		logger.Error("unknown resource type in deployment_changes", "resource_type", change.ResourceType)
		return &ctrlv1.DeploymentChangeEvent{Version: change.Pk}, nil
	}
}

// deploymentRow holds the common fields from both full sync and incremental query results.
type deploymentRow struct {
	dt              db.DeploymentTopology
	d               db.Deployment
	resource        *db.DeploymentResource
	k8sNamespace    sql.NullString
	environmentSlug string
	regionName      string
	gitRepo         sql.NullString
}

// deploymentRowToState converts a deployment row to a proto DeploymentState message.
func deploymentRowToState(row deploymentRow, version uint64) (*ctrlv1.DeploymentState, error) {
	k8sName := row.d.K8sName
	resourceID := ""
	if row.resource != nil {
		if !row.resource.K8sName.Valid || row.resource.K8sName.String == "" {
			return nil, fmt.Errorf("deployment resource %q has no kubernetes name", row.resource.ID)
		}
		k8sName = row.resource.K8sName.String
		resourceID = row.resource.ID
	}

	switch row.dt.DesiredStatus {
	case db.DeploymentTopologyDesiredStatusStopped:
		return &ctrlv1.DeploymentState{
			Version: version,
			State: &ctrlv1.DeploymentState_Delete{
				Delete: &ctrlv1.DeleteDeployment{
					K8SNamespace: row.k8sNamespace.String,
					K8SName:      k8sName,
					ResourceId:   resourceID,
				},
			},
		}, nil
	case db.DeploymentTopologyDesiredStatusRunning:
		var buildID *string
		if row.d.BuildID.Valid {
			buildID = &row.d.BuildID.String
		}
		image := row.d.Image.String
		command := []string(row.d.Command)
		port := row.d.Port
		cpuMillicores := row.d.CpuMillicores
		memoryMib := row.d.MemoryMib
		storageMib := row.d.StorageMib
		resourceName := ""
		resourceKind := ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_UNSPECIFIED
		public := true
		schedule := ""
		var runtime *string
		var handler *string
		if row.resource != nil {
			if err := json.Unmarshal(row.resource.Command, &command); err != nil {
				return nil, fmt.Errorf("failed to unmarshal command for deployment resource %q: %w", row.resource.ID, err)
			}
			image = row.resource.Image.String
			port = row.resource.Port
			cpuMillicores = row.resource.CpuMillicores
			memoryMib = row.resource.MemoryMib
			storageMib = row.resource.StorageMib
			resourceName = row.resource.Name
			resourceKind = deploymentResourceKindToProto(row.resource.Kind)
			public = row.resource.Public
			if row.resource.Schedule.Valid {
				schedule = row.resource.Schedule.String
			}
			if row.resource.Runtime.Valid {
				runtime = &row.resource.Runtime.String
			}
			if row.resource.Handler.Valid {
				handler = &row.resource.Handler.String
			}
		}

		apply := &ctrlv1.ApplyDeployment{
			DeploymentId:                  row.d.ID,
			ResourceId:                    resourceID,
			ResourceName:                  resourceName,
			ResourceKind:                  resourceKind,
			Public:                        public,
			Schedule:                      schedule,
			Runtime:                       runtime,
			Handler:                       handler,
			K8SNamespace:                  row.k8sNamespace.String,
			K8SName:                       k8sName,
			WorkspaceId:                   row.d.WorkspaceID,
			ProjectId:                     row.d.ProjectID,
			EnvironmentId:                 row.d.EnvironmentID,
			AppId:                         row.d.AppID,
			Image:                         image,
			CpuMillicores:                 int64(cpuMillicores),
			MemoryMib:                     int64(memoryMib),
			EncryptedEnvironmentVariables: row.d.EncryptedEnvironmentVariables,
			BuildId:                       buildID,
			Command:                       command,
			Port:                          port,
			ShutdownSignal:                string(row.d.ShutdownSignal),
			EnvironmentSlug:               &row.environmentSlug,
			Region:                        &row.regionName,
		}

		if row.d.GitCommitSha.Valid {
			apply.GitCommitSha = &row.d.GitCommitSha.String
		}
		if row.d.GitBranch.Valid {
			apply.GitBranch = &row.d.GitBranch.String
		}
		if row.d.GitCommitMessage.Valid {
			apply.GitCommitMessage = &row.d.GitCommitMessage.String
		}
		if row.gitRepo.Valid {
			apply.GitRepo = &row.gitRepo.String
		}

		if row.d.Healthcheck.Valid && (row.resource == nil || row.resource.Kind == db.DeploymentResourcesKindService || row.resource.Kind == db.DeploymentResourcesKindFunction) {
			hcBytes, err := json.Marshal(row.d.Healthcheck.Healthcheck)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal healthcheck: %w", err)
			}
			apply.Healthcheck = hcBytes
		}

		policy := &ctrlv1.AutoscalingPolicy{
			MinReplicas: row.dt.AutoscalingReplicasMin,
			MaxReplicas: row.dt.AutoscalingReplicasMax,
		}
		if row.dt.AutoscalingThresholdCpu.Valid {
			policy.CpuThreshold = ptr.P(int32(row.dt.AutoscalingThresholdCpu.Int16))
		}
		if row.dt.AutoscalingThresholdMemory.Valid {
			policy.MemoryThreshold = ptr.P(int32(row.dt.AutoscalingThresholdMemory.Int16))
		}
		apply.Autoscaling = policy

		if storageMib > 0 {
			apply.EphemeralStorage = &ctrlv1.EphemeralStorage{
				SizeMib: int64(storageMib),
			}
		}

		return &ctrlv1.DeploymentState{
			Version: version,
			State: &ctrlv1.DeploymentState_Apply{
				Apply: apply,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown DeploymentTopologyDesiredStatus: %v", row.dt.DesiredStatus)
	}
}

func deploymentResourceKindToProto(kind db.DeploymentResourcesKind) ctrlv1.DeploymentResourceKind {
	switch kind {
	case db.DeploymentResourcesKindService:
		return ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_SERVICE
	case db.DeploymentResourcesKindFunction:
		return ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_FUNCTION
	case db.DeploymentResourcesKindWorker:
		return ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_WORKER
	case db.DeploymentResourcesKindCron:
		return ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_CRON
	case db.DeploymentResourcesKindStatic:
		return ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_UNSPECIFIED
	default:
		return ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_UNSPECIFIED
	}
}
