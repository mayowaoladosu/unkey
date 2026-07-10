package deployment

import (
	"fmt"
	"strings"

	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymanifest"
)

func compileDeploymentManifest(
	context deploymentContext,
	request *hydrav1.DeployRequest,
) (deploymanifest.Compiled, string, db.DeploymentManifestsOutputMode, error) {
	var source deploymanifest.Source
	var build deploymanifest.Build

	switch requestSource := request.GetSource().(type) {
	case *hydrav1.DeployRequest_DockerImage:
		source = deploymanifest.Source{
			Kind:        deploymanifest.SourceKindDockerImage,
			DockerImage: requestSource.DockerImage.GetImage(),
		}
		build = deploymanifest.Build{Strategy: deploymanifest.BuildStrategyPrebuilt}
	case *hydrav1.DeployRequest_Git:
		git := requestSource.Git
		source = deploymanifest.Source{
			Kind:           deploymanifest.SourceKindGit,
			Repository:     git.GetRepository(),
			CommitSHA:      git.GetCommitSha(),
			Branch:         git.GetBranch(),
			ContextPath:    git.GetContextPath(),
			ForkRepository: git.GetForkRepository(),
		}
		if strings.TrimSpace(git.GetDockerfilePath()) == "" {
			build = deploymanifest.Build{
				Strategy:     deploymanifest.BuildStrategyRailpack,
				BuildCommand: strings.TrimSpace(git.GetBuildCommand()),
			}
		} else {
			build = deploymanifest.Build{
				Strategy:   deploymanifest.BuildStrategyDockerfile,
				Dockerfile: strings.TrimSpace(git.GetDockerfilePath()),
			}
		}
	default:
		return deploymanifest.Compiled{}, "", "", fmt.Errorf("deployment source is required")
	}

	routes := []deploymanifest.RouteIntent{{Kind: deploymanifest.RouteKindDeployment}}
	if source.CommitSHA != "" {
		routes = append(routes, deploymanifest.RouteIntent{Kind: deploymanifest.RouteKindCommit})
	}
	if source.Branch != "" {
		routes = append(routes, deploymanifest.RouteIntent{Kind: deploymanifest.RouteKindBranch})
	}
	routes = append(routes, deploymanifest.RouteIntent{Kind: deploymanifest.RouteKindEnvironment})
	if context.env.Environment.Slug == "production" {
		routes = append(routes, deploymanifest.RouteIntent{Kind: deploymanifest.RouteKindLive})
	}

	provenance := deploymanifest.Provenance{}
	if context.appliedFrameworkDetection != nil {
		provenance.DetectionFingerprint = context.appliedFrameworkDetection.Fingerprint
		if context.appliedFrameworkDetection.DetectedPresetID.Valid {
			provenance.FrameworkPreset = context.appliedFrameworkDetection.DetectedPresetID.String
		}
	}

	outputName := context.app.Slug
	if outputName == "" {
		outputName = "default"
	}

	compiled, err := deploymanifest.Compile(deploymanifest.Plan{
		Source: source,
		Build:  build,
		Outputs: []deploymanifest.Output{
			{
				Kind:             deploymanifest.OutputKindContainer,
				Name:             outputName,
				Port:             context.appRuntimeSettings.Port,
				UpstreamProtocol: string(context.appRuntimeSettings.UpstreamProtocol),
			},
		},
		Runtime: deploymanifest.Runtime{
			CpuMillicores:  context.appRuntimeSettings.CpuMillicores,
			MemoryMib:      context.appRuntimeSettings.MemoryMib,
			StorageMib:     context.appRuntimeSettings.StorageMib,
			ShutdownSignal: string(context.appRuntimeSettings.ShutdownSignal),
			Command:        request.GetCommand(),
		},
		Routes:     routes,
		Provenance: provenance,
	})
	if err != nil {
		return deploymanifest.Compiled{}, "", "", fmt.Errorf("compile deployment manifest: %w", err)
	}

	return compiled, "container", db.DeploymentManifestsOutputModeContainer, nil
}
