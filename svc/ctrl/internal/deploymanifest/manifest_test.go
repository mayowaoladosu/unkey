package deploymanifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileProducesCanonicalImmutableManifest(t *testing.T) {
	compiled, err := Compile(Plan{
		Source: Source{
			Kind:        SourceKindGit,
			Repository:  "Layerrail/deploy-hello",
			CommitSHA:   "0123456789abcdef0123456789abcdef01234567",
			Branch:      "main",
			ContextPath: ".",
		},
		Build: Build{
			Strategy:     BuildStrategyRailpack,
			BuildCommand: "pnpm run build",
		},
		Outputs: []Output{
			{
				Kind:             OutputKindContainer,
				Name:             "web",
				Port:             8080,
				UpstreamProtocol: "http1",
			},
		},
		Runtime: Runtime{
			CpuMillicores:  250,
			MemoryMib:      256,
			StorageMib:     0,
			ShutdownSignal: "SIGTERM",
		},
		Routes: []RouteIntent{
			{Kind: RouteKindLive},
			{Kind: RouteKindDeployment},
			{Kind: RouteKindEnvironment},
			{Kind: RouteKindBranch},
		},
		Provenance: Provenance{
			FrameworkPreset:      "vite",
			DetectionFingerprint: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		},
	})
	require.NoError(t, err)

	require.JSONEq(t, `{
		"version": 1,
		"source": {
			"kind": "git",
			"repository": "Layerrail/deploy-hello",
			"commitSha": "0123456789abcdef0123456789abcdef01234567",
			"branch": "main",
			"contextPath": "."
		},
		"build": {
			"strategy": "railpack",
			"buildCommand": "pnpm run build"
		},
		"outputs": [{
			"kind": "container",
			"name": "web",
			"port": 8080,
			"upstreamProtocol": "http1"
		}],
		"runtime": {
			"cpuMillicores": 250,
			"memoryMib": 256,
			"storageMib": 0,
			"shutdownSignal": "SIGTERM"
		},
		"routes": [
			{"kind": "deployment"},
			{"kind": "branch"},
			{"kind": "environment"},
			{"kind": "live"}
		],
		"provenance": {
			"frameworkPreset": "vite",
			"detectionFingerprint": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		}
	}`, string(compiled.JSON))
	require.Equal(t, "2184e0e935333793af5a4244ded7051bae1a68e7053df0495c9f3e63947e62f4", compiled.Fingerprint)
	require.Equal(t, SchemaVersion, compiled.Manifest.Version)

	parsed, err := Parse(compiled.JSON)
	require.NoError(t, err)
	require.Equal(t, compiled.Manifest, parsed)
}

func TestCompileSupportsOneImageServicesWorkersAndCron(t *testing.T) {
	compiled, err := Compile(Plan{
		Source: Source{
			Kind:       SourceKindGit,
			Repository: "Layerrail/commerce",
			CommitSHA:  "0123456789abcdef0123456789abcdef01234567",
		},
		Build: Build{Strategy: BuildStrategyRailpack},
		Outputs: []Output{
			{
				Kind:             OutputKindContainer,
				Name:             "web",
				Port:             3000,
				UpstreamProtocol: "http1",
				Command:          []string{"node", "server.js"},
				Public:           true,
			},
			{
				Kind:    OutputKindWorker,
				Name:    "emails",
				Command: []string{"node", "worker.js"},
			},
			{
				Kind:     OutputKindCron,
				Name:     "cleanup",
				Command:  []string{"node", "cleanup.js"},
				Schedule: "0 * * * *",
			},
		},
		Runtime: Runtime{
			CpuMillicores:  250,
			MemoryMib:      256,
			ShutdownSignal: "SIGTERM",
		},
	})
	require.NoError(t, err)
	require.Len(t, compiled.Manifest.Outputs, 3)
	require.Equal(t, OutputKindContainer, compiled.Manifest.Outputs[0].Kind)
	require.Equal(t, OutputKindCron, compiled.Manifest.Outputs[1].Kind)
	require.Equal(t, OutputKindWorker, compiled.Manifest.Outputs[2].Kind)

	_, err = Compile(Plan{
		Source: Source{Kind: SourceKindDockerImage, DockerImage: "example.com/app:latest"},
		Build:  Build{Strategy: BuildStrategyPrebuilt},
		Outputs: []Output{
			{Kind: OutputKindContainer, Name: "web", Port: 3000, Public: true},
			{Kind: OutputKindContainer, Name: "admin", Port: 3001, Public: true},
		},
	})
	require.ErrorContains(t, err, "at most one public output")

	_, err = Compile(Plan{
		Source: Source{Kind: SourceKindDockerImage, DockerImage: "example.com/app:latest"},
		Build:  Build{Strategy: BuildStrategyPrebuilt},
		Outputs: []Output{
			{Kind: OutputKindWorker, Name: "worker"},
		},
	})
	require.ErrorContains(t, err, "requires a command")
}
