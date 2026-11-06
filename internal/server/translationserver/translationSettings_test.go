package translationserver

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/translate"
)

func TestTranslationSettings(t *testing.T) {
	ctx, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM settings WHERE key = ?`, models.SettingTranslationStyle)
		db.Exec(`DELETE FROM translation_servers`)
	})

	box, _ := secret.New("test-secret")

	var sent []translate.Request

	server := &Server{db: db, box: box, newTranslator: func(translate.Endpoint) translate.Translator {
		return answerFunc(func(_ context.Context, req translate.Request) (string, error) {
			sent = append(sent, req)

			return "Änderungen speichern", nil
		})
	}}

	update := func(style string) (*pb.TranslationSettings, error) {
		res, err := server.UpdateTranslationSettings(ctx, connect.NewRequest(&pb.UpdateTranslationSettingsRequest{Style: style}))
		if err != nil {
			return nil, err
		}

		return res.Msg.Settings, nil
	}

	res, err := server.GetTranslationSettings(ctx, connect.NewRequest(&pb.GetTranslationSettingsRequest{}))
	if err != nil {
		t.Fatal(err)
	}

	if got := res.Msg.Settings; got.Style != "" || got.DefaultStyle != translate.DefaultStyle || got.UpdatedAt != nil {
		t.Errorf("before any change: got %+v, want the default", got)
	}

	saved, err := update("  website of a bakery ")
	if err != nil {
		t.Fatal(err)
	}

	if saved.Style != "website of a bakery" || saved.UpdatedAt == nil {
		t.Errorf("saved: got %+v", saved)
	}

	for _, style := range []string{"two\nlines", "[bracketed]", strings.Repeat("x", maxStyleLength+1)} {
		if _, err := update(style); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%q: got %v, want invalid argument", style, err)
		}
	}

	// The server test translates in the saved style
	created, err := server.CreateTranslationServer(ctx, connect.NewRequest(&pb.CreateTranslationServerRequest{
		Name: "Local", BaseUrl: "http://llama:8080/v1", Model: "hy-mt2", MaxConcurrency: 1, Enabled: true,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := server.TestTranslationServer(ctx, connect.NewRequest(&pb.TestTranslationServerRequest{Id: created.Msg.TranslationServer.Id})); err != nil {
		t.Fatal(err)
	}

	if len(sent) != 1 || sent[0].Style != "website of a bakery" {
		t.Errorf("test sample: got %+v, want the saved style", sent)
	}

	// Empty restores the default
	if saved, err = update(""); err != nil || saved.Style != "" {
		t.Errorf("reset: got %+v, %v", saved, err)
	}
}
