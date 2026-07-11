package deployment

import (
	"context"
	"time"

	"connectrpc.com/connect"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/conc"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/pkg/repeat"
	"github.com/unkeyed/unkey/svc/krane/pkg/labels"
	"github.com/unkeyed/unkey/svc/krane/pkg/metrics"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// runActualStateResyncLoop periodically reports actual instance state to the
// control plane for every deployment ReplicaSet.
//
// This is a fast, lightweight safety net that complements [Controller.runPodWatchLoop].
// The watch handles real-time events, but can miss updates during network partitions,
// restarts, or buffer overflows. This loop catches the drift by rebuilding and
// reporting status for every RS every 30 seconds.
//
// This loop does NOT fetch or apply desired state — that is handled independently
// by [Controller.runDesiredStateResyncLoop] so that slow control plane RPCs cannot
// delay instance reporting.
func (c *Controller) runActualStateResyncLoop(ctx context.Context) {
	repeat.Every(30*time.Second, func() {
		logger.Info("running actual state resync")
		c.forEachReplicaSet(ctx, func(ctx context.Context, rs *appsv1.ReplicaSet) {
			status, err := c.buildDeploymentStatus(ctx, rs)
			if err != nil {
				logger.Error("actual state resync: unable to build deployment status", "error", err.Error(), "replicaSet", rs.Name)
				return
			}
			reported, err := c.reportIfChanged(ctx, status)
			if err != nil {
				logger.Error("actual state resync: unable to report deployment status", "error", err.Error(), "replicaSet", rs.Name)
				return
			}
			if reported {
				// Resync found drift the watch didn't deliver. This is the
				// "pod watch missed an event" smoking-gun signal — a
				// healthy cluster should see this counter stay flat.
				metrics.ResyncCorrectionsTotal.WithLabelValues("deployment").Inc()
				logger.Info("actual state resync: reported changed deployment status", "replicaSet", rs.Name)
			}
		})
	})
}

// runDesiredStateResyncLoop periodically reconciles every deployment ReplicaSet
// against the control plane's desired state.
//
// This is a consistency safety net that complements the streaming desired state
// channel. It runs every minute, fetching the desired state for each RS and
// applying or deleting as needed. Because this involves potentially slow RPCs
// (GetDesiredDeploymentState), it runs independently from actual state reporting
// so it cannot delay instance updates.
func (c *Controller) runDesiredStateResyncLoop(ctx context.Context) {
	repeat.Every(1*time.Minute, func() {
		logger.Info("running desired state resync")
		c.forEachReplicaSet(ctx, func(ctx context.Context, rs *appsv1.ReplicaSet) {
			c.reconcileDesiredState(ctx, rs)
		})
		c.forEachCronJob(ctx, func(ctx context.Context, cron *batchv1.CronJob) {
			c.reconcileDesiredResource(ctx, cron.GetNamespace(), cron.GetName(), cron.GetLabels())
		})
	})
}

func (c *Controller) forEachCronJob(ctx context.Context, fn func(ctx context.Context, cron *batchv1.CronJob)) {
	cursor := ""
	for {
		cronJobs, err := c.clientSet.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{
			LabelSelector: labels.New().
				ManagedByKrane().
				ComponentDeployment().
				ToString(),
			Continue: cursor,
		})
		if err != nil {
			logger.Error("unable to list cron jobs", "error", err.Error())
			return
		}
		conc.ForEach(ctx, cronJobs.Items, fn)
		cursor = cronJobs.Continue
		if cursor == "" {
			return
		}
	}
}

// forEachReplicaSet paginates through all krane-managed deployment ReplicaSets
// and calls fn for each one concurrently.
func (c *Controller) forEachReplicaSet(ctx context.Context, fn func(ctx context.Context, rs *appsv1.ReplicaSet)) {
	cursor := ""
	for {
		replicaSets, err := c.clientSet.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{
			LabelSelector: labels.New().
				ManagedByKrane().
				ComponentDeployment().
				ToString(),
			Continue: cursor,
		})
		if err != nil {
			logger.Error("unable to list replicaSets", "error", err.Error())
			return
		}

		conc.ForEach(ctx, replicaSets.Items, fn)

		cursor = replicaSets.Continue
		if cursor == "" {
			break
		}
	}
}

// reconcileDesiredState fetches the desired state for a single ReplicaSet from
// the control plane and applies or deletes as needed.
func (c *Controller) reconcileDesiredState(ctx context.Context, replicaSet *appsv1.ReplicaSet) {
	c.reconcileDesiredResource(ctx, replicaSet.GetNamespace(), replicaSet.GetName(), replicaSet.GetLabels())
}

func (c *Controller) reconcileDesiredResource(ctx context.Context, namespace, name string, objectLabels map[string]string) {
	deploymentID, ok := labels.GetDeploymentID(objectLabels)
	if !ok {
		logger.Error("unable to get deployment ID", "resource", name)
		return
	}
	resourceID, _ := labels.GetResourceID(objectLabels)

	res, err := c.cluster.GetDesiredDeploymentState(ctx, &ctrlv1.GetDesiredDeploymentStateRequest{
		Region:       c.regionKey(),
		DeploymentId: deploymentID,
		ResourceId:   resourceID,
	})
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			if err := c.DeleteDeployment(ctx, &ctrlv1.DeleteDeployment{
				K8SNamespace: namespace,
				K8SName:      name,
				ResourceId:   resourceID,
			}); err != nil {
				logger.Error("unable to delete deployment", "error", err.Error(), "deployment_id", deploymentID)
			}

			return
		}

		logger.Error("unable to get desired deployment state", "error", err.Error(), "deployment_id", deploymentID)
		return
	}

	switch res.GetState().(type) {
	case *ctrlv1.DeploymentState_Apply:
		if err := c.ApplyDeployment(ctx, res.GetApply()); err != nil {
			logger.Error("unable to apply deployment", "error", err.Error(), "deployment_id", deploymentID)
		}
	case *ctrlv1.DeploymentState_Delete:
		if err := c.DeleteDeployment(ctx, res.GetDelete()); err != nil {
			logger.Error("unable to delete deployment", "error", err.Error(), "deployment_id", deploymentID)
		}
	}
}
