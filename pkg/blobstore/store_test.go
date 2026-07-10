package blobstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
)

func TestMemoryStoreRoundTripReturnsIndependentBytes(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	original := []byte("immutable artifact")

	require.NoError(t, store.Put(ctx, "deployments/d_1/static.tar.gz", original, Metadata{
		ContentType:  "application/gzip",
		CacheControl: "public, max-age=31536000, immutable",
	}))
	original[0] = 'X'

	object, found, err := store.Get(ctx, "deployments/d_1/static.tar.gz")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("immutable artifact"), object.Body)
	require.Equal(t, "application/gzip", object.Metadata.ContentType)
	require.NotEmpty(t, object.ETag)

	object.Body[0] = 'Y'
	again, found, err := store.Get(ctx, "deployments/d_1/static.tar.gz")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("immutable artifact"), again.Body)
}

func TestS3StoreRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Config := containers.S3(t)
	store, err := NewS3(ctx, S3Config{
		Endpoint:        s3Config.URL,
		Region:          "us-east-1",
		Bucket:          "deployment-artifacts",
		AccessKeyID:     s3Config.AccessKeyID,
		SecretAccessKey: s3Config.SecretAccessKey,
		UsePathStyle:    true,
		CreateBucket:    true,
	})
	require.NoError(t, err)

	key := "deployments/d_1/static.tar.gz"
	require.NoError(t, store.Put(ctx, key, []byte("bundle"), Metadata{ContentType: "application/gzip"}))
	object, found, err := store.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("bundle"), object.Body)
	require.Equal(t, "application/gzip", object.Metadata.ContentType)

	_, found, err = store.Get(ctx, "missing")
	require.NoError(t, err)
	require.False(t, found)
}
