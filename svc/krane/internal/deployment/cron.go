package deployment

import (
	"context"
	"encoding/json"
	"fmt"

	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c *Controller) applyCronJob(ctx context.Context, req *ctrlv1.ApplyDeployment, hasSecrets bool) error {
	desired := c.buildCronJob(req, hasSecrets)
	patch, err := json.Marshal(desired)
	if err != nil {
		return fmt.Errorf("failed to marshal cron job: %w", err)
	}
	applied, err := c.clientSet.BatchV1().CronJobs(req.GetK8SNamespace()).Patch(
		ctx,
		req.GetK8SName(),
		types.ApplyPatchType,
		patch,
		metav1.PatchOptions{FieldManager: fieldManagerKrane},
	)
	if err != nil {
		return fmt.Errorf("failed to apply cron job: %w", err)
	}
	if hasSecrets {
		if err := c.patchOwnerRef(ctx, req.GetK8SNamespace(), workloadResourcePrefix(req), cronJobOwnerRef(applied)); err != nil {
			return fmt.Errorf("failed to patch cron owner references: %w", err)
		}
	}
	return c.reportDeploymentStatus(ctx, cronJobStatus(req))
}

func cronJobStatus(req *ctrlv1.ApplyDeployment) *ctrlv1.ReportDeploymentStatusRequest {
	return &ctrlv1.ReportDeploymentStatusRequest{
		Change: &ctrlv1.ReportDeploymentStatusRequest_Update_{
			Update: &ctrlv1.ReportDeploymentStatusRequest_Update{
				K8SName:    req.GetK8SName(),
				ResourceId: req.GetResourceId(),
				Instances: []*ctrlv1.ReportDeploymentStatusRequest_Update_Instance{{
					K8SName:       req.GetK8SName(),
					Address:       "cronjob://" + req.GetK8SName(),
					CpuMillicores: req.GetCpuMillicores(),
					MemoryMib:     req.GetMemoryMib(),
					Status:        ctrlv1.ReportDeploymentStatusRequest_Update_Instance_STATUS_RUNNING,
				}},
			},
		},
	}
}

func (c *Controller) buildCronJob(req *ctrlv1.ApplyDeployment, hasSecrets bool) *batchv1.CronJob {
	template := c.buildReplicaSet(req, hasSecrets).Spec.Template
	template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	template.Spec.TopologySpreadConstraints = nil
	startingDeadline := int64(300)
	successHistory := int32(3)
	failedHistory := int32(3)

	return &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "CronJob"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.GetK8SName(),
			Namespace: req.GetK8SNamespace(),
			Labels:    deploymentLabels(req),
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   req.GetSchedule(),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			StartingDeadlineSeconds:    &startingDeadline,
			SuccessfulJobsHistoryLimit: &successHistory,
			FailedJobsHistoryLimit:     &failedHistory,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: deploymentLabels(req)},
				Spec: batchv1.JobSpec{
					BackoffLimit: ptrInt32(3),
					Template:     template,
				},
			},
		},
	}
}

func cronJobOwnerRef(cron *batchv1.CronJob) metav1.OwnerReference {
	controller := true
	blockOwnerDeletion := true
	return metav1.OwnerReference{
		APIVersion:         "batch/v1",
		Kind:               "CronJob",
		Name:               cron.Name,
		UID:                cron.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

func ptrInt32(value int32) *int32 {
	return &value
}
