package objectstore_test

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/kazuto/parlance-server/internal/objectstore"
	"github.com/kazuto/parlance-server/internal/objectstore/objectstoretest"
)

func TestS3(t *testing.T) {
	ctx := context.Background()
	fake := objectstoretest.NewS3(t)
	store := objectstore.NewS3(fake.Config(), nil)

	t.Run("puts objects with their hash", func(t *testing.T) {
		err := store.Put(ctx, objectstore.Object{
			Key:          "i18n/en.abc.json",
			Body:         []byte(`{"greeting": "Hello"}`),
			ContentType:  "application/json",
			CacheControl: "public, max-age=31536000, immutable",
			SHA256:       "abc",
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		object, ok := fake.Object("i18n/en.abc.json")
		if !ok || string(object.Body) != `{"greeting": "Hello"}` || object.ContentType != "application/json" ||
			object.CacheControl != "public, max-age=31536000, immutable" || object.Metadata["sha256"] != "abc" {
			t.Errorf("object = %+v", object)
		}
	})

	t.Run("stats objects", func(t *testing.T) {
		stat, err := store.Stat(ctx, "i18n/en.abc.json")
		if err != nil || !stat.Exists || stat.SHA256 != "abc" {
			t.Errorf("Stat() = %+v, %v", stat, err)
		}

		stat, err = store.Stat(ctx, "i18n/missing.json")
		if err != nil || stat.Exists {
			t.Errorf("Stat() of a missing key = %+v, %v", stat, err)
		}
	})

	t.Run("gets objects", func(t *testing.T) {
		data, err := store.Get(ctx, "i18n/en.abc.json")
		if err != nil || string(data) != `{"greeting": "Hello"}` {
			t.Errorf("Get() = %q, %v", data, err)
		}

		if _, err := store.Get(ctx, "i18n/missing.json"); !stderrors.Is(err, objectstore.ErrNotFound) {
			t.Errorf("Get() of a missing key error = %v, want ErrNotFound", err)
		}
	})

	t.Run("checks access and cleans up", func(t *testing.T) {
		if err := store.Check(ctx, "i18n"); err != nil {
			t.Fatalf("Check() error = %v", err)
		}

		if _, ok := fake.Object("i18n/.parlance-check"); ok {
			t.Error("Check() left its object")
		}

		calls := fake.Calls()
		if want := []string{"HEAD", "PUT i18n/.parlance-check", "DELETE i18n/.parlance-check"}; strings.Join(calls[len(calls)-3:], ",") != strings.Join(want, ",") {
			t.Errorf("calls = %v, want them to end with %v", calls, want)
		}
	})
}

func TestS3Errors(t *testing.T) {
	ctx := context.Background()
	fake := objectstoretest.NewS3(t)

	cases := []struct {
		name   string
		change func(*objectstore.Config)
		want   string
	}{
		{"missing bucket", func(c *objectstore.Config) { c.Bucket = "other" }, `bucket "other" does not exist`},
		{"unknown access key", func(c *objectstore.Config) { c.AccessKeyID = "wrong" }, "denied access"},
		{"unreachable storage", func(c *objectstore.Config) { c.Endpoint = "http://127.0.0.1:1" }, "can't reach the storage"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := fake.Config()
			tc.change(&config)

			err := objectstore.NewS3(config, nil).Check(ctx, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check() error = %v, want %q", err, tc.want)
			}
		})
	}

	t.Run("explains failed writes", func(t *testing.T) {
		store := objectstore.NewS3(fake.Config(), nil)

		config := fake.Config()
		config.AccessKeyID = "wrong"

		err := objectstore.NewS3(config, nil).Put(ctx, objectstore.Object{Key: "a.json"})
		if err == nil || err.Error() != "the storage doesn't know the access key ID" {
			t.Errorf("Put() with a wrong key error = %v", err)
		}

		fake.SetReadOnly(true)
		defer fake.SetReadOnly(false)

		err = store.Put(ctx, objectstore.Object{Key: "a.json"})
		if err == nil || !strings.Contains(err.Error(), "denied access") {
			t.Errorf("Put() while read-only error = %v", err)
		}

		if err := store.Check(ctx, ""); err == nil || !strings.Contains(err.Error(), "can't write .parlance-check") {
			t.Errorf("Check() while read-only error = %v", err)
		}
	})
}
