// Package staticassets resolves immutable deployment bundles from object
// storage and keeps a bounded in-process archive cache at the edge.
package staticassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/unkeyed/unkey/pkg/blobstore"
	"github.com/unkeyed/unkey/pkg/staticbundle"
)

type ArtifactRef struct {
	StorageKey  string
	Digest      string
	SPAFallback bool
}

type Config struct {
	Store      blobstore.Store
	MaxEntries int
}

type Resolver interface {
	Resolve(ctx context.Context, artifact ArtifactRef, requestPath string) (staticbundle.File, bool, error)
}

type cacheEntry struct {
	archive  *staticbundle.Archive
	sequence uint64
}

type service struct {
	store      blobstore.Store
	maxEntries int

	mu       sync.Mutex
	sequence uint64
	cache    map[string]cacheEntry
}

func New(config Config) (Resolver, error) {
	if config.Store == nil {
		return nil, fmt.Errorf("static artifact store is required")
	}
	if config.MaxEntries < 1 {
		return nil, fmt.Errorf("static artifact cache size must be positive")
	}
	return &service{
		store:      config.Store,
		maxEntries: config.MaxEntries,
		cache:      make(map[string]cacheEntry, config.MaxEntries),
	}, nil
}

func (s *service) Resolve(
	ctx context.Context,
	artifact ArtifactRef,
	requestPath string,
) (staticbundle.File, bool, error) {
	archive, err := s.load(ctx, artifact)
	if err != nil {
		return staticbundle.File{}, false, err
	}
	file, found := archive.Resolve(requestPath, artifact.SPAFallback)
	return file, found, nil
}

func (s *service) load(ctx context.Context, artifact ArtifactRef) (*staticbundle.Archive, error) {
	if artifact.StorageKey == "" || len(artifact.Digest) != sha256.Size*2 {
		return nil, fmt.Errorf("static artifact reference is invalid")
	}
	cacheKey := artifact.StorageKey + "@" + artifact.Digest

	s.mu.Lock()
	entry, found := s.cache[cacheKey]
	if found {
		s.sequence++
		entry.sequence = s.sequence
		s.cache[cacheKey] = entry
		s.mu.Unlock()
		return entry.archive, nil
	}
	s.mu.Unlock()

	object, found, err := s.store.Get(ctx, artifact.StorageKey, staticbundle.DefaultLimits().MaxArchiveBytes)
	if err != nil {
		return nil, fmt.Errorf("load static artifact: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("static artifact %q was not found", artifact.StorageKey)
	}
	digest := sha256.Sum256(object.Body)
	actualDigest := hex.EncodeToString(digest[:])
	if actualDigest != artifact.Digest {
		return nil, fmt.Errorf("static artifact digest mismatch: expected %s, got %s", artifact.Digest, actualDigest)
	}
	archive, err := staticbundle.Open(object.Body, staticbundle.DefaultLimits())
	if err != nil {
		return nil, fmt.Errorf("open static artifact: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	if len(s.cache) >= s.maxEntries {
		var oldestKey string
		oldestSequence := ^uint64(0)
		for key, candidate := range s.cache {
			if candidate.sequence < oldestSequence {
				oldestKey = key
				oldestSequence = candidate.sequence
			}
		}
		delete(s.cache, oldestKey)
	}
	s.cache[cacheKey] = cacheEntry{archive: archive, sequence: s.sequence}
	return archive, nil
}
