package staticassets

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/blobstore"
	"github.com/unkeyed/unkey/pkg/staticbundle"
)

type countingStore struct {
	blobstore.Store
	gets int
}

func (s *countingStore) Get(ctx context.Context, key string, maxBytes int64) (blobstore.Object, bool, error) {
	s.gets++
	return s.Store.Get(ctx, key, maxBytes)
}

func TestResolverVerifiesAndCachesImmutableStaticBundle(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(root+"/index.html", []byte("<h1>LayerRail</h1>"), 0o644))
	bundle, err := staticbundle.PackDirectory(root, staticbundle.DefaultLimits())
	require.NoError(t, err)

	memory := blobstore.NewMemory()
	require.NoError(t, memory.Put(context.Background(), "bundle.tar.gz", bundle.Bytes, blobstore.Metadata{}))
	store := &countingStore{Store: memory}
	resolver, err := New(Config{Store: store, MaxEntries: 2})
	require.NoError(t, err)

	ref := ArtifactRef{StorageKey: "bundle.tar.gz", Digest: bundle.Digest, SPAFallback: true}
	file, found, err := resolver.Resolve(context.Background(), ref, "/dashboard")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("<h1>LayerRail</h1>"), file.Body)

	_, _, err = resolver.Resolve(context.Background(), ref, "/")
	require.NoError(t, err)
	require.Equal(t, 1, store.gets, "immutable bundle should be downloaded once")

	_, _, err = resolver.Resolve(context.Background(), ArtifactRef{
		StorageKey: "bundle.tar.gz",
		Digest:     "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}, "/")
	require.ErrorContains(t, err, "digest")
}
