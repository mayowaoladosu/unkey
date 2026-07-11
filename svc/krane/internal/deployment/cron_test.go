package deployment

import (
	"testing"

	"github.com/stretchr/testify/require"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/svc/krane/pkg/labels"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildCronJobMaterializesScheduledCommand(t *testing.T) {
	req := fullApplyRequest(t)
	req.ResourceKind = ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_CRON
	req.Public = false
	req.Port = 0
	req.Schedule = "17 * * * *"
	req.Command = []string{"node", "cleanup.js"}
	req.Healthcheck = nil

	cron := testController().buildCronJob(req, false)
	require.Equal(t, "17 * * * *", cron.Spec.Schedule)
	require.Equal(t, batchv1.ForbidConcurrent, cron.Spec.ConcurrencyPolicy)
	require.Equal(t, corev1.RestartPolicyOnFailure, cron.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy)
	require.Equal(t, []string{"node", "cleanup.js"}, cron.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command)
	require.Empty(t, cron.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Ports)
	require.Equal(t, testResourceID, cron.Labels[labels.LabelKeyResourceID])
	require.Equal(t, "cron", cron.Labels[labels.LabelKeyResourceKind])
	status := cronJobStatus(req).GetUpdate()
	require.Equal(t, testResourceID, status.GetResourceId())
	require.Len(t, status.GetInstances(), 1)
	require.Equal(t, ctrlv1.ReportDeploymentStatusRequest_Update_Instance_STATUS_RUNNING, status.GetInstances()[0].GetStatus())
}
