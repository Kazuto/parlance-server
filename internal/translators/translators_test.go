package translators

import (
	"context"
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/translate"
)

func TestDBSource(t *testing.T) {
	db, err := testutil.SetupTestDB()
	if err != nil {
		t.Fatalf("SetupTestDB: %v", err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM translation_servers`) })
	ctx := context.Background()

	box, _ := secret.New("test-secret")
	var built []translate.Endpoint
	source := &DBSource{db: db, box: box, pools: translate.NewPools(func(endpoint translate.Endpoint) translate.Translator {
		built = append(built, endpoint)
		return nil
	})}

	t.Run("has no translator without enabled servers", func(t *testing.T) {
		db.Create(&models.TranslationServer{Name: "Off", BaseURL: "http://off/v1", MaxConcurrency: 1})
		translator, err := source.Translator(ctx)
		if err != nil || translator != nil {
			t.Errorf("got %v, %v", translator, err)
		}
		if Enabled(ctx, db) {
			t.Error("Enabled: got true")
		}
	})

	t.Run("pools the enabled servers with their keys", func(t *testing.T) {
		key, _ := box.Encrypt("sk-123")
		db.Create(&models.TranslationServer{Name: "A", BaseURL: "http://a/v1", APIKey: key, MaxConcurrency: 3, Enabled: true})
		db.Create(&models.TranslationServer{Name: "B", BaseURL: "http://b/v1", MaxConcurrency: 1, Enabled: true})
		// Encrypted with another key, so it can't be used.
		other, _ := secret.New("other-secret")
		broken, _ := other.Encrypt("sk-456")
		db.Create(&models.TranslationServer{Name: "Broken", BaseURL: "http://broken/v1", APIKey: broken, MaxConcurrency: 1, Enabled: true})

		built = nil
		first, err := source.Translator(ctx)
		if err != nil || first == nil {
			t.Fatalf("got %v, %v", first, err)
		}
		if len(built) != 2 {
			t.Fatalf("built %d endpoints: %+v", len(built), built)
		}
		urls := map[string]translate.Endpoint{}
		for _, endpoint := range built {
			urls[endpoint.BaseURL] = endpoint
		}
		if urls["http://a/v1"].APIKey != "sk-123" || urls["http://a/v1"].MaxConcurrency != 3 {
			t.Errorf("a: got %+v", urls["http://a/v1"])
		}
		if _, ok := urls["http://broken/v1"]; ok {
			t.Error("used a server whose key can't be decrypted")
		}
		if !Enabled(ctx, db) {
			t.Error("Enabled: got false")
		}

		// The same settings keep the same pool, so limits hold across requests.
		second, _ := source.Translator(ctx)
		if second != first || len(built) != 2 {
			t.Errorf("expected the same pool")
		}
	})
}
