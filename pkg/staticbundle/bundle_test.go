package staticbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBundleIsDeterministicAndResolvesViteAssets(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("<main>LayerRail</main>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "assets", "app.01234567.js"), []byte("console.log('ready')"), 0o644))

	first, err := PackDirectory(root, DefaultLimits())
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(filepath.Join(root, "index.html"), time.Now(), time.Now()))
	second, err := PackDirectory(root, DefaultLimits())
	require.NoError(t, err)
	require.Equal(t, first.Digest, second.Digest)
	require.Equal(t, first.Bytes, second.Bytes)
	require.Equal(t, 2, first.FileCount)

	archive, err := Open(first.Bytes, DefaultLimits())
	require.NoError(t, err)

	index, found := archive.Resolve("/", true)
	require.True(t, found)
	require.Equal(t, []byte("<main>LayerRail</main>"), index.Body)
	require.Equal(t, "text/html; charset=utf-8", index.ContentType)
	require.Equal(t, "no-cache", index.CacheControl)

	asset, found := archive.Resolve("/assets/app.01234567.js", true)
	require.True(t, found)
	require.Equal(t, []byte("console.log('ready')"), asset.Body)
	require.Contains(t, asset.ContentType, "javascript")
	require.Equal(t, "public, max-age=31536000, immutable", asset.CacheControl)
	require.NotEmpty(t, asset.ETag)

	spa, found := archive.Resolve("/dashboard/settings", true)
	require.True(t, found)
	require.Equal(t, index.Body, spa.Body)

	_, found = archive.Resolve("../../etc/passwd", true)
	require.False(t, found)
}

func TestOpenRejectsArchivePathTraversal(t *testing.T) {
	var encoded bytes.Buffer
	gzipWriter := gzip.NewWriter(&encoded)
	tarWriter := tar.NewWriter(gzipWriter)
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{
		Name:     "../secret.txt",
		Mode:     0o644,
		Size:     6,
		Typeflag: tar.TypeReg,
	}))
	_, err := tarWriter.Write([]byte("secret"))
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	_, err = Open(encoded.Bytes(), DefaultLimits())
	require.ErrorContains(t, err, "unsafe path")
}
