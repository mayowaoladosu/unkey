// Package deploymanifest compiles deployment intent into a deterministic,
// immutable manifest shared by the control plane and materializers.
package deploymanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

const SchemaVersion = 1

type SourceKind string

const (
	SourceKindGit         SourceKind = "git"
	SourceKindDockerImage SourceKind = "docker_image"
)

type BuildStrategy string

const (
	BuildStrategyPrebuilt   BuildStrategy = "prebuilt"
	BuildStrategyDockerfile BuildStrategy = "dockerfile"
	BuildStrategyRailpack   BuildStrategy = "railpack"
	BuildStrategyStatic     BuildStrategy = "static"
)

type OutputKind string

const (
	OutputKindContainer OutputKind = "container"
	OutputKindStatic    OutputKind = "static"
	OutputKindFunction  OutputKind = "function"
	OutputKindWorker    OutputKind = "worker"
)

type RouteKind string

const (
	RouteKindDeployment  RouteKind = "deployment"
	RouteKindCommit      RouteKind = "commit"
	RouteKindBranch      RouteKind = "branch"
	RouteKindEnvironment RouteKind = "environment"
	RouteKindLive        RouteKind = "live"
)

type Source struct {
	Kind           SourceKind `json:"kind"`
	Repository     string     `json:"repository,omitempty"`
	CommitSHA      string     `json:"commitSha,omitempty"`
	Branch         string     `json:"branch,omitempty"`
	ContextPath    string     `json:"contextPath,omitempty"`
	DockerImage    string     `json:"dockerImage,omitempty"`
	ForkRepository string     `json:"forkRepository,omitempty"`
}

type Build struct {
	Strategy              BuildStrategy `json:"strategy"`
	Dockerfile            string        `json:"dockerfile,omitempty"`
	BuildCommand          string        `json:"buildCommand,omitempty"`
	StaticOutputDirectory string        `json:"staticOutputDirectory,omitempty"`
}

type Output struct {
	Kind             OutputKind `json:"kind"`
	Name             string     `json:"name"`
	Port             int32      `json:"port,omitempty"`
	UpstreamProtocol string     `json:"upstreamProtocol,omitempty"`
	Directory        string     `json:"directory,omitempty"`
	Runtime          string     `json:"runtime,omitempty"`
	Handler          string     `json:"handler,omitempty"`
}

type Runtime struct {
	CpuMillicores  int32    `json:"cpuMillicores"`
	MemoryMib      int32    `json:"memoryMib"`
	StorageMib     uint32   `json:"storageMib"`
	ShutdownSignal string   `json:"shutdownSignal"`
	Command        []string `json:"command,omitempty"`
}

type RouteIntent struct {
	Kind RouteKind `json:"kind"`
}

type Provenance struct {
	FrameworkPreset      string `json:"frameworkPreset,omitempty"`
	DetectionFingerprint string `json:"detectionFingerprint,omitempty"`
}

type Plan struct {
	Source     Source
	Build      Build
	Outputs    []Output
	Runtime    Runtime
	Routes     []RouteIntent
	Provenance Provenance
}

type Manifest struct {
	Version    int           `json:"version"`
	Source     Source        `json:"source"`
	Build      Build         `json:"build"`
	Outputs    []Output      `json:"outputs"`
	Runtime    Runtime       `json:"runtime"`
	Routes     []RouteIntent `json:"routes"`
	Provenance Provenance    `json:"provenance"`
}

type Compiled struct {
	Manifest    Manifest
	JSON        []byte
	Fingerprint string
}

// Parse decodes a persisted manifest and rejects unsupported versions or
// structurally invalid deployment intent before materialization.
func Parse(encoded []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode deployment manifest: %w", err)
	}
	if manifest.Version != SchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported deployment manifest version %d", manifest.Version)
	}
	if err := validate(Plan{
		Source:     manifest.Source,
		Build:      manifest.Build,
		Outputs:    manifest.Outputs,
		Runtime:    manifest.Runtime,
		Routes:     manifest.Routes,
		Provenance: manifest.Provenance,
	}); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Compile validates deployment intent, canonicalizes set-like fields, and
// returns a stable JSON document plus its SHA-256 fingerprint.
func Compile(plan Plan) (Compiled, error) {
	if err := validate(plan); err != nil {
		return Compiled{}, err
	}

	outputs := slices.Clone(plan.Outputs)
	slices.SortFunc(outputs, func(a, b Output) int {
		if a.Kind != b.Kind {
			return compare(string(a.Kind), string(b.Kind))
		}
		return compare(a.Name, b.Name)
	})

	routes := slices.Clone(plan.Routes)
	slices.SortFunc(routes, func(a, b RouteIntent) int {
		return routeRank(a.Kind) - routeRank(b.Kind)
	})

	manifest := Manifest{
		Version:    SchemaVersion,
		Source:     plan.Source,
		Build:      plan.Build,
		Outputs:    outputs,
		Runtime:    plan.Runtime,
		Routes:     routes,
		Provenance: plan.Provenance,
	}

	encoded, err := json.Marshal(manifest)
	if err != nil {
		return Compiled{}, fmt.Errorf("marshal deployment manifest: %w", err)
	}

	digest := sha256.Sum256(encoded)
	return Compiled{
		Manifest:    manifest,
		JSON:        encoded,
		Fingerprint: hex.EncodeToString(digest[:]),
	}, nil
}

func validate(plan Plan) error {
	switch plan.Source.Kind {
	case SourceKindGit:
		if plan.Source.Repository == "" || plan.Source.CommitSHA == "" {
			return fmt.Errorf("git source requires repository and commit SHA")
		}
	case SourceKindDockerImage:
		if plan.Source.DockerImage == "" {
			return fmt.Errorf("docker image source requires an image")
		}
	default:
		return fmt.Errorf("unsupported source kind %q", plan.Source.Kind)
	}

	switch plan.Build.Strategy {
	case BuildStrategyPrebuilt:
		if plan.Source.Kind != SourceKindDockerImage {
			return fmt.Errorf("prebuilt strategy requires a docker image source")
		}
	case BuildStrategyDockerfile:
		if plan.Source.Kind != SourceKindGit || plan.Build.Dockerfile == "" {
			return fmt.Errorf("dockerfile strategy requires a git source and dockerfile")
		}
	case BuildStrategyRailpack:
		if plan.Source.Kind != SourceKindGit {
			return fmt.Errorf("railpack strategy requires a git source")
		}
	case BuildStrategyStatic:
		if plan.Source.Kind != SourceKindGit || plan.Build.StaticOutputDirectory == "" {
			return fmt.Errorf("static strategy requires a git source and output directory")
		}
	default:
		return fmt.Errorf("unsupported build strategy %q", plan.Build.Strategy)
	}

	if len(plan.Outputs) == 0 {
		return fmt.Errorf("deployment manifest requires at least one output")
	}
	for _, output := range plan.Outputs {
		if output.Name == "" {
			return fmt.Errorf("deployment output requires a name")
		}
		switch output.Kind {
		case OutputKindContainer:
			if output.Port < 1 || output.Port > 65535 {
				return fmt.Errorf("container output %q requires a valid port", output.Name)
			}
		case OutputKindStatic:
			if output.Directory == "" {
				return fmt.Errorf("static output %q requires a directory", output.Name)
			}
		case OutputKindFunction:
			if output.Runtime == "" || output.Handler == "" {
				return fmt.Errorf("function output %q requires runtime and handler", output.Name)
			}
		case OutputKindWorker:
			// Workers are materialized by a runtime adapter and require no route.
		default:
			return fmt.Errorf("unsupported output kind %q", output.Kind)
		}
	}
	return nil
}

func routeRank(kind RouteKind) int {
	switch kind {
	case RouteKindDeployment:
		return 0
	case RouteKindCommit:
		return 1
	case RouteKindBranch:
		return 2
	case RouteKindEnvironment:
		return 3
	case RouteKindLive:
		return 4
	default:
		return 100
	}
}

func compare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
