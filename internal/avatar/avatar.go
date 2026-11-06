// Package avatar keeps users' avatars in S3-compatible storage and serves them
// through the API at /avatars/{id}, so the bucket needs no public access and its
// address needn't be reachable from browsers.
package avatar

import (
	"context"
	stderrors "errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/kazuto/parlance-server/internal/errors"
)

// MaxSize caps an avatar image's size.
const MaxSize = 2 << 20

// pathPrefix is where the API serves avatars.
const pathPrefix = "/avatars/"

// types are the image types avatars may have; others, SVG among them, could carry
// scripts or aren't images.
var types = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}

// userIDPattern accepts the ids avatars are stored under, and nothing that could
// reach elsewhere in the bucket.
var userIDPattern = regexp.MustCompile(`^[0-9a-fA-F-]{1,64}$`)

// ErrNotFound means there is no avatar under the key.
var ErrNotFound = stderrors.New("avatar not found")

// Storage keeps avatar images by key.
type Storage interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Get returns the image and its type, or ErrNotFound.
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, key string) error
}

// Store saves avatars and serves them.
type Store struct {
	storage Storage
	// now tells the time, which versions the addresses; tests replace it.
	now func() time.Time
}

// New creates a store keeping avatars in the storage; nil when none is configured,
// so avatars can't be changed.
func New(storage Storage) *Store {
	return &Store{storage: storage, now: time.Now}
}

// Save stores the user's avatar and returns its address to keep on the user,
// which changes with every new image so browsers don't show an old one. The
// image's type is read from its bytes, not taken from the client.
func (s *Store) Save(ctx context.Context, userID string, data []byte) (string, error) {
	if s.storage == nil {
		return "", errors.FailedPrecondition(errors.ErrFailedPrecondition, "avatars need S3 storage, which the server has not configured")
	}

	if len(data) > MaxSize {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "the avatar is larger than 2 MB")
	}

	contentType := http.DetectContentType(data)
	if !types[contentType] {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "the avatar must be a PNG, JPEG, WebP or GIF image")
	}

	if err := s.storage.Put(ctx, key(userID), data, contentType); err != nil {
		return "", errors.InternalCause(err, "failed to store the avatar")
	}

	return pathPrefix + userID + "?v=" + strconv.FormatInt(s.now().Unix(), 10), nil
}

// Remove deletes the user's avatar, if any.
func (s *Store) Remove(ctx context.Context, userID string) error {
	if s.storage == nil {
		return nil
	}

	if err := s.storage.Delete(ctx, key(userID)); err != nil {
		return errors.InternalCause(err, "failed to remove the avatar")
	}

	return nil
}

// ServeHTTP serves the avatar of the user in the path's {id}. Avatars are public,
// as images can't send the sign-in token and user ids can't be guessed.
func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if s.storage == nil || !userIDPattern.MatchString(userID) {
		http.NotFound(w, r)

		return
	}

	body, contentType, err := s.storage.Get(r.Context(), key(userID))
	if stderrors.Is(err, ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if err != nil {
		log.Printf("avatars: load %s: %v", userID, err)
		http.Error(w, "the avatar can't be loaded", http.StatusBadGateway)

		return
	}

	defer func() { _ = body.Close() }()

	// Only ever shown as an image, also when someone opens it directly.
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := io.Copy(w, body); err != nil {
		log.Printf("avatars: send %s: %v", userID, err)
	}
}

func key(userID string) string {
	return "avatars/" + userID
}
