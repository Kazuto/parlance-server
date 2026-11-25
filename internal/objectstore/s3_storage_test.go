package objectstore_test

import (
	"context"
	stderrors "errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/kazuto/parlance-server/internal/objectstore"
)

// storageURLEnv names the variable holding a real S3-compatible storage for
// integration tests, as "http://ACCESS_KEY:SECRET_KEY@localhost:9000/bucket". The
// bucket must exist; the tests write below "parlance-test/".
const storageURLEnv = "PARLANCE_TEST_S3_URL"

func storageConfig(t *testing.T) objectstore.Config {
	t.Helper()

	raw := os.Getenv(storageURLEnv)
	if raw == "" {
		t.Skipf("%s not set", storageURLEnv)
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		t.Fatalf("%s must look like http://ACCESS_KEY:SECRET_KEY@host/bucket", storageURLEnv)
	}

	secretKey, _ := parsed.User.Password()

	return objectstore.Config{
		Endpoint:        parsed.Scheme + "://" + parsed.Host,
		Region:          parsed.Query().Get("region"),
		Bucket:          strings.Trim(parsed.Path, "/"),
		AccessKeyID:     parsed.User.Username(),
		SecretAccessKey: secretKey,
		PathStyle:       true,
	}
}

func TestS3Storage(t *testing.T) {
	ctx := context.Background()
	config := storageConfig(t)
	store := objectstore.NewS3(config, nil)

	if err := store.Check(ctx, "parlance-test"); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	object := objectstore.Object{
		Key:          "parlance-test/en.json",
		Body:         []byte("{\n  \"greeting\": \"Hello\"\n}\n"),
		ContentType:  "application/json; charset=utf-8",
		CacheControl: "public, max-age=31536000, immutable",
		SHA256:       "e3b0c442",
	}

	if err := store.Put(ctx, object); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if stat, err := store.Stat(ctx, object.Key); err != nil || !stat.Exists || stat.SHA256 != object.SHA256 {
		t.Errorf("Stat() = %+v, %v", stat, err)
	}

	if stat, err := store.Stat(ctx, "parlance-test/missing.json"); err != nil || stat.Exists {
		t.Errorf("Stat() of a missing key = %+v, %v", stat, err)
	}

	if data, err := store.Get(ctx, object.Key); err != nil || string(data) != string(object.Body) {
		t.Errorf("Get() = %q, %v", data, err)
	}

	if _, err := store.Get(ctx, "parlance-test/missing.json"); !stderrors.Is(err, objectstore.ErrNotFound) {
		t.Errorf("Get() of a missing key error = %v", err)
	}

	wrongSecret := config
	wrongSecret.SecretAccessKey += "x"

	if err := objectstore.NewS3(wrongSecret, nil).Put(ctx, object); err == nil || !strings.Contains(err.Error(), "secret access key") {
		t.Errorf("Put() with a wrong secret key error = %v", err)
	}

	wrongBucket := config
	wrongBucket.Bucket += "-missing"

	if err := objectstore.NewS3(wrongBucket, nil).Check(ctx, ""); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Check() of a missing bucket error = %v", err)
	}
}
