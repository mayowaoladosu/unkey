package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymanifest"
)

func TestBuildStaticExtractionDockerfileUsesBoundedAppPath(t *testing.T) {
	dockerfile, err := buildStaticExtractionDockerfile(
		"registry.example.com/project:deployment",
		"dist",
	)
	require.NoError(t, err)
	require.Equal(t, `# syntax=docker/dockerfile:1.7
FROM registry.example.com/project:deployment AS application
FROM scratch
COPY --from=application /app/dist/ /
`, dockerfile)

	_, err = buildStaticExtractionDockerfile("registry.example.com/project:deployment", "../secrets")
	require.ErrorContains(t, err, "relative directory")
}

func TestIsStaticOnlyDeploymentDoesNotCollapseMixedOutputs(t *testing.T) {
	require.True(t, isStaticOnlyDeployment(deploymanifest.Manifest{Outputs: []deploymanifest.Output{
		{Kind: deploymanifest.OutputKindStatic},
	}}))
	require.False(t, isStaticOnlyDeployment(deploymanifest.Manifest{Outputs: []deploymanifest.Output{
		{Kind: deploymanifest.OutputKindStatic},
		{Kind: deploymanifest.OutputKindContainer},
	}}))
}

func TestBuildPlainStaticDockerfileExportsRepositoryRoot(t *testing.T) {
	require.Equal(t, `# syntax=docker/dockerfile:1.7
FROM scratch
COPY . /
`, buildPlainStaticDockerfile())
}
