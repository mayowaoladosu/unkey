// Package blobstore provides immutable object storage used by deployment
// materializers and Frontline.
package blobstore

import "context"

type Metadata struct {
	ContentType  string
	CacheControl string
}

type Object struct {
	Body     []byte
	Metadata Metadata
	ETag     string
}

type Store interface {
	Put(ctx context.Context, key string, body []byte, metadata Metadata) error
	Get(ctx context.Context, key string, maxBytes int64) (Object, bool, error)
}
