// Package objectstoretest fakes an S3-compatible storage for tests.
package objectstoretest

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kazuto/parlance-server/internal/objectstore"
)

// Object is an object in the fake bucket.
type Object struct {
	Body         []byte
	ContentType  string
	CacheControl string
	// Metadata holds the x-amz-meta-* headers, by their lowercase names without the prefix.
	Metadata map[string]string
}

// S3 fakes the parts of the S3 API that objectstore.S3 uses, for one bucket in
// memory, addressed by path. It checks the access key ID but not the signature.
type S3 struct {
	Server          *httptest.Server
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string

	mu       sync.Mutex
	objects  map[string]Object
	readOnly bool
	// calls lists "METHOD key" of each request, with "" as the key of the bucket.
	calls []string
}

// NewS3 starts a fake with the empty bucket "translations". It stops with the test.
func NewS3(t *testing.T) *S3 {
	t.Helper()

	f := &S3{
		Bucket:          "translations",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		objects:         map[string]Object{},
	}

	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)

	return f
}

// Config is how to reach the fake.
func (f *S3) Config() objectstore.Config {
	return objectstore.Config{
		Endpoint:        f.Server.URL,
		Bucket:          f.Bucket,
		AccessKeyID:     f.AccessKeyID,
		SecretAccessKey: f.SecretAccessKey,
		PathStyle:       true,
	}
}

// SetReadOnly makes writes and deletes fail with AccessDenied, as for a key that
// may only read.
func (f *S3) SetReadOnly(readOnly bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.readOnly = readOnly
}

// Put stores an object, e.g. one an earlier export left.
func (f *S3) Put(key string, object Object) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.objects[key] = object
}

// Object returns the object at the key.
func (f *S3) Object(key string) (Object, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	object, ok := f.objects[key]

	return object, ok
}

// Keys lists the keys of the objects, sorted.
func (f *S3) Keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Sorted(maps.Keys(f.objects))
}

// Calls lists "METHOD key" of each request so far.
func (f *S3) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

func (f *S3) serve(w http.ResponseWriter, r *http.Request) {
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")

	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, strings.TrimSpace(r.Method+" "+key))

	if accessKeyID(r) != f.AccessKeyID {
		writeError(w, r, http.StatusForbidden, "InvalidAccessKeyId", "The AWS Access Key Id you provided does not exist in our records.")

		return
	}

	if bucket != f.Bucket {
		writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")

		return
	}

	if key == "" {
		if r.Method != http.MethodHead {
			writeError(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", "Only HEAD is faked for buckets")

			return
		}

		w.WriteHeader(http.StatusOK)

		return
	}

	switch r.Method {
	case http.MethodHead, http.MethodGet:
		f.read(w, r, key)
	case http.MethodPut:
		f.write(w, r, key)
	case http.MethodDelete:
		if f.readOnly {
			writeError(w, r, http.StatusForbidden, "AccessDenied", "Access Denied")

			return
		}

		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", "Not faked")
	}
}

func (f *S3) read(w http.ResponseWriter, r *http.Request, key string) {
	object, ok := f.objects[key]
	if !ok {
		writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")

		return
	}

	header := w.Header()
	header.Set("Content-Length", strconv.Itoa(len(object.Body)))

	if object.ContentType != "" {
		header.Set("Content-Type", object.ContentType)
	}

	if object.CacheControl != "" {
		header.Set("Cache-Control", object.CacheControl)
	}

	for name, value := range object.Metadata {
		header.Set("X-Amz-Meta-"+name, value)
	}

	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodGet {
		_, _ = w.Write(object.Body)
	}
}

func (f *S3) write(w http.ResponseWriter, r *http.Request, key string) {
	if f.readOnly {
		writeError(w, r, http.StatusForbidden, "AccessDenied", "Access Denied")

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "IncompleteBody", err.Error())

		return
	}

	object := Object{
		Body:         body,
		ContentType:  r.Header.Get("Content-Type"),
		CacheControl: r.Header.Get("Cache-Control"),
		Metadata:     map[string]string{},
	}

	for name, values := range r.Header {
		if lower := strings.ToLower(name); strings.HasPrefix(lower, "x-amz-meta-") && len(values) > 0 {
			object.Metadata[strings.TrimPrefix(lower, "x-amz-meta-")] = values[0]
		}
	}

	f.objects[key] = object

	w.Header().Set("ETag", fmt.Sprintf("%q", strconv.Itoa(len(f.objects))))
	w.WriteHeader(http.StatusOK)
}

// accessKeyID reads the access key ID from a SigV4 Authorization header.
func accessKeyID(r *http.Request) string {
	_, credential, ok := strings.Cut(r.Header.Get("Authorization"), "Credential=")
	if !ok {
		return ""
	}

	id, _, _ := strings.Cut(credential, "/")

	return id
}

// writeError answers as S3 does: with an XML error, or only the status for HEAD.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if r.Method == http.MethodHead {
		w.WriteHeader(status)

		return
	}

	var body bytes.Buffer

	fmt.Fprintf(&body, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, message)

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
