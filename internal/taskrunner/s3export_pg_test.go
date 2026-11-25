package taskrunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/objectstore/objectstoretest"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))

	return hex.EncodeToString(sum[:])
}

func TestS3Export(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	en, de, _ := seedLocales(t, db.DB)

	seedEntry(t, db.DB, "greeting", map[string]string{en.ID: "Hello"}, models.StatusApproved)
	seedEntry(t, db.DB, "farewell", map[string]string{en.ID: "Goodbye", de.ID: "Tschüss"}, models.StatusDraft)

	ctx := context.Background()
	fake := objectstoretest.NewS3(t)
	r := newTestRunner(t, db)

	config := models.S3ExportConfig{
		Endpoint:     fake.Server.URL,
		Bucket:       fake.Bucket,
		AccessKeyID:  fake.AccessKeyID,
		PathStyle:    true,
		ManifestPath: "i18n/manifest.json",
		HashLength:   12,
		ExportSelection: models.ExportSelection{
			Files:     []models.ExportFile{{Format: "json", Path: "i18n/{locale}.{hash}.json"}},
			LocaleIDs: []string{en.ID, de.ID},
		},
	}
	task := newTask(t, db.DB, models.TaskTypeS3Export, config)

	enContent := "{\n  \"greeting\": \"Hello\"\n}\n"
	enKey := "i18n/en." + sha256Hex(enContent)[:12] + ".json"
	deKey := "i18n/de." + sha256Hex("{}\n")[:12] + ".json"

	readManifest := func(t *testing.T) manifest {
		t.Helper()

		object, ok := fake.Object("i18n/manifest.json")
		if !ok {
			t.Fatal("no manifest")
		}

		var written manifest
		if err := json.Unmarshal(object.Body, &written); err != nil {
			t.Fatalf("manifest: %v", err)
		}

		return written
	}

	t.Run("fails without a secret key", func(t *testing.T) {
		if _, err := r.s3Export(ctx, &task, &runLog{}); err == nil || !strings.Contains(err.Error(), "no secret access key") {
			t.Errorf("error = %v", err)
		}
	})

	secretKey, _ := r.box.Encrypt(fake.SecretAccessKey)
	task.Secret = secretKey

	t.Run("uploads files named by their hash, then the manifest", func(t *testing.T) {
		summary, err := r.s3Export(ctx, &task, &runLog{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}

		if summary.Outcome != forge.OutcomeUpdated || !slices.Equal(summary.ChangedFiles, []string{deKey, enKey}) ||
			summary.ManifestURL != "s3://translations/i18n/manifest.json" {
			t.Errorf("summary = %+v", summary)
		}

		if keys := fake.Keys(); !slices.Equal(keys, []string{deKey, enKey, "i18n/manifest.json"}) {
			t.Errorf("keys = %v", keys)
		}

		object, _ := fake.Object(enKey)
		if string(object.Body) != enContent || object.Metadata["sha256"] != sha256Hex(enContent) ||
			object.CacheControl != fileCacheControl || object.ContentType != "application/json; charset=utf-8" {
			t.Errorf("%s = %+v, want only the approved greeting, with its hash", enKey, object)
		}

		if calls := fake.Calls(); calls[len(calls)-1] != "PUT i18n/manifest.json" {
			t.Errorf("calls = %v, want the manifest written last", calls)
		}

		if manifestObject, _ := fake.Object("i18n/manifest.json"); manifestObject.CacheControl != manifestCacheControl {
			t.Errorf("manifest Cache-Control = %q", manifestObject.CacheControl)
		}

		written := readManifest(t)
		want := manifestFile{
			Locale:   "en",
			Format:   "json",
			Template: "i18n/{locale}.{hash}.json",
			Key:      enKey,
			SHA256:   sha256Hex(enContent),
			Size:     len(enContent),
		}

		if written.Version != 1 || written.Task != "Task" || written.IncludeDrafts || len(written.Files) != 2 ||
			written.Files[0].Locale != "de" || written.Files[1] != want {
			t.Errorf("manifest = %+v", written)
		}
	})

	t.Run("uploads nothing the second time", func(t *testing.T) {
		before := len(fake.Calls())

		summary, err := r.s3Export(ctx, &task, &runLog{})
		if err != nil || summary.Outcome != forge.OutcomeNoChanges || len(summary.ChangedFiles) != 0 {
			t.Errorf("summary = %+v, error = %v, want no changes", summary, err)
		}

		for _, call := range fake.Calls()[before:] {
			if strings.HasPrefix(call, "PUT") {
				t.Errorf("wrote %s", call)
			}
		}
	})

	t.Run("uploads again what was stored without its hash", func(t *testing.T) {
		generatedAt := readManifest(t).GeneratedAt

		fake.Put(enKey, objectstoretest.Object{Body: []byte("{}")})

		summary, err := r.s3Export(ctx, &task, &runLog{})
		if err != nil || summary.Outcome != forge.OutcomeUpdated || !slices.Equal(summary.ChangedFiles, []string{enKey}) {
			t.Errorf("summary = %+v, error = %v", summary, err)
		}

		if object, _ := fake.Object(enKey); string(object.Body) != enContent {
			t.Errorf("%s = %q", enKey, object.Body)
		}

		if readManifest(t).GeneratedAt != generatedAt {
			t.Error("rewrote the unchanged manifest")
		}
	})

	t.Run("refuses to replace other content under a key", func(t *testing.T) {
		fake.Put(enKey, objectstoretest.Object{Body: []byte("{}"), Metadata: map[string]string{"sha256": sha256Hex("{}")}})
		defer fake.Put(enKey, objectstoretest.Object{Body: []byte(enContent), Metadata: map[string]string{"sha256": sha256Hex(enContent)}})

		if _, err := r.s3Export(ctx, &task, &runLog{}); err == nil || !strings.Contains(err.Error(), "use a longer hash") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("lists changed translations under new keys, keeping the old files", func(t *testing.T) {
		config.IncludeDrafts = true
		encoded, _ := datatypesJSON(config)
		task.Config = encoded

		summary, err := r.s3Export(ctx, &task, &runLog{})
		if err != nil || summary.Outcome != forge.OutcomeUpdated || len(summary.ChangedFiles) != 2 {
			t.Fatalf("summary = %+v, error = %v", summary, err)
		}

		written := readManifest(t)
		if !written.IncludeDrafts || written.Files[1].Key == enKey || !slices.Contains(fake.Keys(), written.Files[1].Key) {
			t.Errorf("manifest = %+v", written)
		}

		if !slices.Contains(fake.Keys(), enKey) {
			t.Errorf("keys = %v, want the old files kept", fake.Keys())
		}
	})

	t.Run("fails with the storage's reason", func(t *testing.T) {
		fake.SetReadOnly(true)
		defer fake.SetReadOnly(false)

		config.IncludeDrafts = false
		config.HashLength = 0
		encoded, _ := datatypesJSON(config)
		task.Config = encoded

		if _, err := r.s3Export(ctx, &task, &runLog{}); err == nil || !strings.Contains(err.Error(), "denied access") {
			t.Errorf("error = %v", err)
		}
	})
}
