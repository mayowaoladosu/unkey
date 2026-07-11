package deploy

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	restate "github.com/restatedev/sdk-go"
	"github.com/tonistiigi/fsutil"
)

var imageReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`)

func buildStaticExtractionDockerfile(imageName, outputDirectory string) (string, error) {
	imageName = strings.TrimSpace(imageName)
	outputDirectory = strings.TrimSpace(outputDirectory)
	if !imageReferencePattern.MatchString(imageName) {
		return "", fmt.Errorf("invalid static source image")
	}
	if !isValidGitContextPath(outputDirectory) || outputDirectory == "" {
		return "", fmt.Errorf("static output must be a relative directory")
	}

	appPath := "/app"
	if outputDirectory != "." {
		appPath = path.Join(appPath, outputDirectory)
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM %s AS application
FROM scratch
COPY --from=application %s/ /
`, imageName, appPath), nil
}

func buildPlainStaticDockerfile() string {
	return `# syntax=docker/dockerfile:1.7
FROM scratch
COPY . /
`
}

func (w *Workflow) buildPlainStaticBundleFromGit(
	ctx restate.Context,
	params gitBuildParams,
) (*buildResult, error) {
	return w.runGitBuild(ctx, "build plain static bundle from git", params, func(runCtx restate.RunContext, buildContext gitBuildContext) (*buildResult, error) {
		workDir, err := os.MkdirTemp("", "static-source-*")
		if err != nil {
			return nil, fmt.Errorf("create static build workspace: %w", err)
		}
		defer func() { _ = os.RemoveAll(workDir) }()

		dockerfileDir := filepath.Join(workDir, "dockerfile")
		outputDir := filepath.Join(workDir, "output")
		for _, dir := range []string{dockerfileDir, outputDir} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create static build directory: %w", err)
			}
		}
		if err := os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte(buildPlainStaticDockerfile()), 0o644); err != nil {
			return nil, fmt.Errorf("write static source Dockerfile: %w", err)
		}

		depotBuildID, err := w.withDepotBuildkit(runCtx, buildContext.DepotProjectID, params, func(buildClient *client.Client) error {
			options, err := w.buildStaticGitExportSolverOptions(
				buildContext.GitContextURL,
				dockerfileDir,
				outputDir,
				buildContext.GithubToken,
			)
			if err != nil {
				return err
			}
			return w.solveWithStatus(runCtx, buildClient, params, options)
		})
		if err != nil {
			return nil, err
		}

		artifact, err := materializeStaticDirectory(runCtx, w.artifactStore, outputDir, staticArtifactIdentity{
			WorkspaceID:  params.WorkspaceID,
			DeploymentID: params.DeploymentID,
			OutputName:   params.StaticOutputName,
			SPAFallback:  params.StaticSPAFallback,
		})
		if err != nil {
			return nil, err
		}
		return &buildResult{
			DepotBuildID:   depotBuildID,
			DepotProjectID: buildContext.DepotProjectID,
			StaticArtifact: artifact,
		}, nil
	})
}

func (w *Workflow) extractStaticBundleFromImage(
	ctx context.Context,
	buildClient *client.Client,
	params gitBuildParams,
	imageName string,
) (*materializedStaticArtifact, error) {
	dockerfile, err := buildStaticExtractionDockerfile(imageName, params.StaticOutputDirectory)
	if err != nil {
		return nil, err
	}
	workDir, err := os.MkdirTemp("", "static-image-*")
	if err != nil {
		return nil, fmt.Errorf("create static extraction workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	dockerfileDir := filepath.Join(workDir, "dockerfile")
	contextDir := filepath.Join(workDir, "context")
	outputDir := filepath.Join(workDir, "output")
	for _, dir := range []string{dockerfileDir, contextDir, outputDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create static extraction directory: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		return nil, fmt.Errorf("write static extraction Dockerfile: %w", err)
	}

	options, err := w.buildStaticImageExportSolverOptions(dockerfileDir, contextDir, outputDir)
	if err != nil {
		return nil, err
	}
	if err := w.solveWithStatus(ctx, buildClient, params, options); err != nil {
		return nil, fmt.Errorf("export static output: %w", err)
	}
	return materializeStaticDirectory(ctx, w.artifactStore, outputDir, staticArtifactIdentity{
		WorkspaceID:  params.WorkspaceID,
		DeploymentID: params.DeploymentID,
		OutputName:   params.StaticOutputName,
		SPAFallback:  params.StaticSPAFallback,
	})
}

func (w *Workflow) buildStaticGitExportSolverOptions(
	gitContextURL, dockerfileDir, outputDir, githubToken string,
) (client.SolveOpt, error) {
	dockerfileFS, err := fsutil.NewFS(dockerfileDir)
	if err != nil {
		return client.SolveOpt{}, fmt.Errorf("create static dockerfile mount: %w", err)
	}
	var attachables []session.Attachable
	if githubToken != "" {
		attachables = append(attachables, secretsprovider.FromMap(map[string][]byte{
			gitAuthTokenSecretID: []byte(githubToken),
		}))
	}
	return client.SolveOpt{
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"platform":      w.buildPlatform.Platform,
			"context":       gitContextURL,
			"filename":      "Dockerfile",
			"dockerfilekey": "dockerfile",
		},
		LocalMounts: map[string]fsutil.FS{"dockerfile": dockerfileFS},
		Session:     attachables,
		Exports: []client.ExportEntry{{
			Type:      client.ExporterLocal,
			OutputDir: outputDir,
		}},
	}, nil
}

func (w *Workflow) buildStaticImageExportSolverOptions(
	dockerfileDir, contextDir, outputDir string,
) (client.SolveOpt, error) {
	dockerfileFS, err := fsutil.NewFS(dockerfileDir)
	if err != nil {
		return client.SolveOpt{}, fmt.Errorf("create extraction dockerfile mount: %w", err)
	}
	contextFS, err := fsutil.NewFS(contextDir)
	if err != nil {
		return client.SolveOpt{}, fmt.Errorf("create extraction context mount: %w", err)
	}
	return client.SolveOpt{
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"platform": w.buildPlatform.Platform,
			"filename": "Dockerfile",
		},
		LocalMounts: map[string]fsutil.FS{
			"dockerfile": dockerfileFS,
			"context":    contextFS,
		},
		Session: []session.Attachable{w.registryAuthProvider()},
		Exports: []client.ExportEntry{{
			Type:      client.ExporterLocal,
			OutputDir: outputDir,
		}},
	}, nil
}
