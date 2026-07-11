package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func TestEvaluateResourceReadinessRequiresEveryResource(t *testing.T) {
	requirements := map[string]map[string]uint32{
		"web":     {"region_a": 1, "region_b": 1},
		"worker":  {"region_a": 1, "region_b": 1},
		"cleanup": {"region_a": 1},
	}
	instances := []db.Instance{
		{RegionID: "region_a", ResourceID: "web", Status: db.InstancesStatusRunning},
		{RegionID: "region_a", ResourceID: "worker", Status: db.InstancesStatusRunning},
		{RegionID: "region_b", ResourceID: "web", Status: db.InstancesStatusRunning},
	}
	ready, _, _ := evaluateResourceReadiness(instances, requirements)
	require.False(t, ready)

	instances = append(instances, db.Instance{RegionID: "region_b", ResourceID: "worker", Status: db.InstancesStatusRunning})
	ready, _, _ = evaluateResourceReadiness(instances, requirements)
	require.False(t, ready, "singleton cron resource still needs its materialization acknowledgement")

	instances = append(instances, db.Instance{RegionID: "region_a", ResourceID: "cleanup", Status: db.InstancesStatusRunning})
	ready, healthy, required := evaluateResourceReadiness(instances, requirements)
	require.True(t, ready)
	require.Equal(t, 1, healthy["cleanup"])
	require.Equal(t, 1, required["cleanup"])
}
