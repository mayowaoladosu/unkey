package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/unkeyed/unkey/pkg/blobstore"
	"github.com/unkeyed/unkey/pkg/staticbundle"
)

type staticArtifactIdentity struct {
	WorkspaceID  string
	DeploymentID string
	OutputName   string
	SPAFallback  bool
}

type staticArtifactMetadata struct {
	SPAFallback       bool  `json:"spaFallback"`
	FileCount         int   `json:"fileCount"`
	UncompressedBytes int64 `json:"uncompressedBytes"`
}

type materializedStaticArtifact struct {
	StorageKey  string
	Digest      string
	SizeBytes   uint64
	ContentType string
	Metadata    []byte
}

var artifactKeySegment = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func materializeStaticDirectory(
	ctx context.Context,
	store blobstore.Store,
	root string,
	identity staticArtifactIdentity,
) (*materializedStaticArtifact, error) {
	if store == nil {
		return nil, fmt.Errorf("static artifact store is not configured")
	}
	for name, value := range map[string]string{
		"workspace_id":  identity.WorkspaceID,
		"deployment_id": identity.DeploymentID,
		"output_name":   identity.OutputName,
	} {
		if !artifactKeySegment.MatchString(value) {
			return nil, fmt.Errorf("invalid static artifact %s %q", name, value)
		}
	}

	bundle, err := staticbundle.PackDirectory(root, staticbundle.DefaultLimits())
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(staticArtifactMetadata{
		SPAFallback:       identity.SPAFallback,
		FileCount:         bundle.FileCount,
		UncompressedBytes: bundle.UncompressedBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("encode static artifact metadata: %w", err)
	}

	storageKey := fmt.Sprintf(
		"deployments/%s/%s/%s/%s.tar.gz",
		identity.WorkspaceID,
		identity.DeploymentID,
		identity.OutputName,
		bundle.Digest,
	)
	const contentType = "application/gzip"
	if err := store.Put(ctx, storageKey, bundle.Bytes, blobstore.Metadata{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}); err != nil {
		return nil, fmt.Errorf("upload static artifact: %w", err)
	}

	return &materializedStaticArtifact{
		StorageKey:  storageKey,
		Digest:      bundle.Digest,
		SizeBytes:   uint64(len(bundle.Bytes)),
		ContentType: contentType,
		Metadata:    metadata,
	}, nil
}
