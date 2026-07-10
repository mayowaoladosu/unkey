package deploy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/blobstore"
	"github.com/unkeyed/unkey/pkg/staticbundle"
)

func TestMaterializeStaticDirectoryUploadsResolvableBundle(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("<h1>deployed</h1>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "assets", "app.01234567.js"), []byte("ready"), 0o644))

	store := blobstore.NewMemory()
	artifact, err := materializeStaticDirectory(context.Background(), store, root, staticArtifactIdentity{
		WorkspaceID:  "ws_test",
		DeploymentID: "d_test",
		OutputName:   "web",
		SPAFallback:  true,
	})
	require.NoError(t, err)
	require.Contains(t, artifact.StorageKey, "deployments/ws_test/d_test/web/")
	require.Contains(t, artifact.StorageKey, artifact.Digest)
	require.Equal(t, "application/gzip", artifact.ContentType)
	require.Positive(t, artifact.SizeBytes)

	stored, found, err := store.Get(context.Background(), artifact.StorageKey)
	require.NoError(t, err)
	require.True(t, found)
	archive, err := staticbundle.Open(stored.Body, staticbundle.DefaultLimits())
	require.NoError(t, err)
	file, found := archive.Resolve("/dashboard", true)
	require.True(t, found)
	require.Equal(t, []byte("<h1>deployed</h1>"), file.Body)

	var metadata staticArtifactMetadata
	require.NoError(t, json.Unmarshal(artifact.Metadata, &metadata))
	require.True(t, metadata.SPAFallback)
	require.Equal(t, 2, metadata.FileCount)
}
