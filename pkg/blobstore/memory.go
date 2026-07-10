package blobstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

type memoryStore struct {
	mu      sync.RWMutex
	objects map[string]Object
}

func NewMemory() Store {
	return &memoryStore{objects: make(map[string]Object)}
}

func (s *memoryStore) Put(_ context.Context, key string, body []byte, metadata Metadata) error {
	digest := sha256.Sum256(body)
	object := Object{
		Body:     append([]byte(nil), body...),
		Metadata: metadata,
		ETag:     hex.EncodeToString(digest[:]),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = object
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) (Object, bool, error) {
	s.mu.RLock()
	object, found := s.objects[key]
	s.mu.RUnlock()
	if !found {
		return Object{}, false, nil
	}
	object.Body = append([]byte(nil), object.Body...)
	return object, true, nil
}
