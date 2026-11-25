// Package objectstore writes files to S3-compatible buckets.
package objectstore

import (
	"context"
	stderrors "errors"
)

// ErrNotFound tells that the bucket holds no object at a key.
var ErrNotFound = stderrors.New("object not found")

// Config tells how to reach a bucket.
type Config struct {
	// Endpoint is the storage's address, e.g. "https://minio.example.com"; empty for
	// Amazon S3.
	Endpoint string
	// Region is empty for "us-east-1".
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	// PathStyle puts the bucket into the path instead of the host name, as MinIO needs.
	PathStyle bool
}

// Object is a file to store.
type Object struct {
	Key          string
	Body         []byte
	ContentType  string
	CacheControl string
	// SHA256 is the hex SHA-256 of Body, kept in the object's metadata.
	SHA256 string
}

// Stat is what the bucket holds at a key.
type Stat struct {
	Exists bool
	// SHA256 is the hash kept in the object's metadata; empty for objects stored
	// without one.
	SHA256 string
}

// Store is a bucket.
type Store interface {
	// Check tells whether the bucket can be read and written under the prefix, by
	// writing a small object there. It removes the object again if it may.
	Check(ctx context.Context, prefix string) error
	// Stat tells whether the bucket holds an object at the key.
	Stat(ctx context.Context, key string) (Stat, error)
	// Get returns the object at the key, or ErrNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
	// Put stores the object, replacing what the key held.
	Put(ctx context.Context, object Object) error
}
