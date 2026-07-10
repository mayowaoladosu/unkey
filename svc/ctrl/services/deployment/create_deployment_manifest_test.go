package deployment

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymanifest"
)

func TestCompileDeploymentManifestMapsGitIntentAndAcceptedDetection(t *testing.T) {
	context := deploymentContext{
		workspaceID: "ws_test",
		project:     db.Project{ID: "prj_test"},
		app:         db.App{ID: "app_test", Slug: "web"},
		env: db.FindEnvironmentByAppIdAndSlugRow{Environment: db.Environment{
			ID:   "env_test",
			Slug: "production",
		}},
		appRuntimeSettings: db.AppRuntimeSetting{
			Port:             8080,
			CpuMillicores:    250,
			MemoryMib:        256,
			StorageMib:       0,
			ShutdownSignal:   db.AppRuntimeSettingsShutdownSignalSIGTERM,
			UpstreamProtocol: db.AppRuntimeSettingsUpstreamProtocolHttp1,
		},
		appliedFrameworkDetection: &db.FindAppliedFrameworkDetectionRow{
			Fingerprint:      "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			DetectionVersion: 1,
			DetectedPresetID: sql.NullString{String: "vite", Valid: true},
		},
	}
	request := &hydrav1.DeployRequest{
		DeploymentId: "d_test",
		Command:      []string{"node", "server.js"},
		Source: &hydrav1.DeployRequest_Git{Git: &hydrav1.GitSource{
			Repository:     "Layerrail/deploy-hello",
			CommitSha:      "0123456789abcdef0123456789abcdef01234567",
			Branch:         "main",
			ContextPath:    ".",
			BuildCommand:   "pnpm run build",
			DockerfilePath: "",
		}},
	}

	compiled, adapterID, outputMode, err := compileDeploymentManifest(context, request)
	require.NoError(t, err)
	require.Equal(t, "container", adapterID)
	require.Equal(t, db.DeploymentManifestsOutputModeContainer, outputMode)
	require.Equal(t, deploymanifest.SourceKindGit, compiled.Manifest.Source.Kind)
	require.Equal(t, deploymanifest.BuildStrategyRailpack, compiled.Manifest.Build.Strategy)
	require.Equal(t, "vite", compiled.Manifest.Provenance.FrameworkPreset)
	require.Equal(t, context.appliedFrameworkDetection.Fingerprint, compiled.Manifest.Provenance.DetectionFingerprint)
	require.Equal(t, []deploymanifest.RouteIntent{
		{Kind: deploymanifest.RouteKindDeployment},
		{Kind: deploymanifest.RouteKindCommit},
		{Kind: deploymanifest.RouteKindBranch},
		{Kind: deploymanifest.RouteKindEnvironment},
		{Kind: deploymanifest.RouteKindLive},
	}, compiled.Manifest.Routes)
	require.Equal(t, []string{"node", "server.js"}, compiled.Manifest.Runtime.Command)
}
