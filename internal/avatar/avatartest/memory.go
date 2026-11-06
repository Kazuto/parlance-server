// Package avatartest keeps avatars in memory for tests.
package avatartest

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/kazuto/parlance-server/internal/avatar"
)

// Memory is an avatar.Storage in memory.
type Memory struct {
	mu     sync.Mutex
	images map[string]image
}

type image struct {
	data        []byte
	contentType string
}

// NewMemory creates an empty storage.
func NewMemory() *Memory {
	return &Memory{images: map[string]image{}}
}

// Put stores the image.
func (m *Memory) Put(_ context.Context, key string, data []byte, contentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.images[key] = image{data: bytes.Clone(data), contentType: contentType}

	return nil
}

// Get returns the image and its type, or avatar.ErrNotFound.
func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stored, ok := m.images[key]
	if !ok {
		return nil, "", avatar.ErrNotFound
	}

	return io.NopCloser(bytes.NewReader(stored.data)), stored.contentType, nil
}

// Delete removes the image.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.images, key)

	return nil
}

// Has tells whether an image is stored under the key.
func (m *Memory) Has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.images[key]

	return ok
}
