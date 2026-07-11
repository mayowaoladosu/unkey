package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymanifest"
)

func TestResourceMappingAndPublicOutputInference(t *testing.T) {
	outputs := []deploymanifest.Output{
		{Kind: deploymanifest.OutputKindWorker, Name: "emails", Command: []string{"node", "worker.js"}},
		{Kind: deploymanifest.OutputKindContainer, Name: "web", Port: 3000},
		{Kind: deploymanifest.OutputKindContainer, Name: "admin", Port: 3001},
	}
	require.Equal(t, "web", inferredPublicOutput(outputs))
	outputs[2].Public = true
	require.Equal(t, "admin", inferredPublicOutput(outputs))

	tests := map[deploymanifest.OutputKind]db.DeploymentResourcesKind{
		deploymanifest.OutputKindContainer: db.DeploymentResourcesKindService,
		deploymanifest.OutputKindFunction:  db.DeploymentResourcesKindFunction,
		deploymanifest.OutputKindWorker:    db.DeploymentResourcesKindWorker,
		deploymanifest.OutputKindCron:      db.DeploymentResourcesKindCron,
		deploymanifest.OutputKindStatic:    db.DeploymentResourcesKindStatic,
	}
	for input, expected := range tests {
		actual, err := resourceKind(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}
