package deploy

import (
	"database/sql"
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
	require.Empty(t, inferredPublicOutput(outputs), "multi-resource manifests require an explicit public output")
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

func TestResolvePrivateBindingsBuildsEndpointAndInboundPolicy(t *testing.T) {
	outputs := []deploymanifest.Output{
		{Kind: deploymanifest.OutputKindContainer, Name: "api", Port: 8080},
		{
			Kind:    deploymanifest.OutputKindWorker,
			Name:    "emails",
			Command: []string{"node", "worker.js"},
			Bindings: []deploymanifest.Binding{{
				Name:     "API",
				Resource: "api",
			}},
		},
	}
	identities := map[string]materializedResourceIdentity{
		"api":    {id: "resource_api", k8sName: sql.NullString{Valid: true, String: "deploy-api"}, port: 8080},
		"emails": {id: "resource_emails", k8sName: sql.NullString{Valid: true, String: "deploy-emails"}},
	}

	bindings, callers, err := resolvePrivateBindings(outputs, identities)
	require.NoError(t, err)
	require.Equal(t, []resolvedPrivateBinding{{
		Name:         "API",
		ResourceID:   "resource_api",
		ResourceName: "api",
		Protocol:     deploymanifest.BindingProtocolHTTP,
		Host:         "deploy-api",
		Port:         8080,
	}}, bindings["emails"])
	require.Equal(t, []string{"resource_emails"}, callers["resource_api"])
}

func TestCronResourcesRunInOneDeterministicRegion(t *testing.T) {
	require.True(t, resourceRunsInRegion(db.DeploymentResourcesKindCron, "region_a", "region_a"))
	require.False(t, resourceRunsInRegion(db.DeploymentResourcesKindCron, "region_b", "region_a"))
	require.True(t, resourceRunsInRegion(db.DeploymentResourcesKindWorker, "region_b", "region_a"))
}
