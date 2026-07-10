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
