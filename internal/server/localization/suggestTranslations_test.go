package localization

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/translate"
)

// fakeTranslator answers from a table by target language, and records requests.
type fakeTranslator struct {
	mu       sync.Mutex
	answers  map[string]string
	requests []translate.Request
}

func (f *fakeTranslator) Translate(_ context.Context, req translate.Request) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	answer, ok := f.answers[req.TargetLang]
	if !ok {
		return "", errors.New("model unavailable")
	}
	return answer, nil
}

// fixedSource always returns the same translator.
type fixedSource struct{ translator translate.Translator }

// hangingTranslator never answers for one language, like an overloaded server.
type hangingTranslator struct {
	translate.Translator
	language string
}

func (h hangingTranslator) Translate(ctx context.Context, req translate.Request) (string, error) {
	if req.TargetLang == h.language {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return h.Translator.Translate(ctx, req)
}

func (f fixedSource) Translator(context.Context) (translate.Translator, error) {
	return f.translator, nil
}

func (f *fakeTranslator) request(lang string) (translate.Request, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, req := range f.requests {
		if req.TargetLang == lang {
			return req, true
		}
	}
	return translate.Request{}, false
}

// seedTranslationData adds locales en (default), de, fr, es and sv, and an entry
// with English text. sv is not a language the models support.
func seedTranslationData(t *testing.T, db *database.DB) {
	t.Helper()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec(`INSERT INTO locales (id, code, is_default) VALUES
		('st-en', 'en', 1), ('st-de', 'de', 0), ('st-fr', 'fr', 0), ('st-es', 'es', 0), ('st-sv', 'sv', 0)`)
	exec(`INSERT INTO entries (id, key, description) VALUES
		('st-entry', 'dashboard.open', 'Button on the start page'),
		('st-empty', 'dashboard.empty', '')`)
	exec(`INSERT INTO localizations (id, entry_id, locale_id, translation) VALUES
		('st-l1', 'st-entry', 'st-en', 'Open the dashboard'),
		('st-l2', 'st-entry', 'st-de', 'Übersicht öffnen')`)
	exec(`INSERT INTO terminologies (id, term) VALUES ('st-t1', 'Dashboard'), ('st-t2', 'Logo')`)
	exec(`INSERT INTO definitions (id, terminology_id, locale_id, translation) VALUES
		('st-d1', 'st-t1', 'st-es', 'Panel de control'),
		('st-d2', 'st-t2', 'st-es', 'Logotipo')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localization_history WHERE entry_id LIKE 'st-%'`)
		db.Exec(`DELETE FROM localizations WHERE entry_id LIKE 'st-%'`)
		db.Exec(`DELETE FROM definitions WHERE id LIKE 'st-%'`)
		db.Exec(`DELETE FROM terminologies WHERE id LIKE 'st-%'`)
		db.Exec(`DELETE FROM entries WHERE id LIKE 'st-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'st-%'`)
	})
}

func TestSuggestTranslations(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}
	seedTranslationData(t, db)

	newServer := func() (*Server, *fakeTranslator) {
		fake := &fakeTranslator{answers: map[string]string{
			"German":  "Übersicht öffnen",
			"Spanish": `"Abrir el panel de control"`,
		}}
		return &Server{db: db, translators: fixedSource{fake}}, fake
	}
	suggest := func(server *Server, ctx context.Context, entryID string, localeIDs ...string) (*pb.SuggestTranslationsResponse, error) {
		res, err := server.SuggestTranslations(ctx, connect.NewRequest(&pb.SuggestTranslationsRequest{
			EntryId:         entryID,
			TargetLocaleIds: localeIDs,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}

	t.Run("suggests one cleaned translation per locale, in order", func(t *testing.T) {
		server, _ := newServer()
		res, err := suggest(server, ctx, "st-entry", "st-es", "st-de")
		if err != nil {
			t.Fatalf("SuggestTranslations: %v", err)
		}
		if len(res.Suggestions) != 2 {
			t.Fatalf("got %d suggestions", len(res.Suggestions))
		}
		es, de := res.Suggestions[0], res.Suggestions[1]
		if es.LocaleId != "st-es" || es.Translation != "Abrir el panel de control" || es.ErrorCode != "" {
			t.Errorf("es: got %+v", es)
		}
		if de.LocaleId != "st-de" || de.Translation != "Übersicht öffnen" {
			t.Errorf("de: got %+v", de)
		}
	})

	t.Run("returns the languages done in time when the call runs out of time", func(t *testing.T) {
		defer func(timeout time.Duration) { suggestTimeout = timeout }(suggestTimeout)
		suggestTimeout = 50 * time.Millisecond
		_, fake := newServer()
		server := &Server{db: db, translators: fixedSource{hangingTranslator{fake, "French"}}}

		res, err := suggest(server, ctx, "st-entry", "st-de", "st-fr")
		if err != nil {
			t.Fatalf("SuggestTranslations: %v", err)
		}
		de, fr := res.Suggestions[0], res.Suggestions[1]
		if de.Translation != "Übersicht öffnen" || de.ErrorCode != "" {
			t.Errorf("de: got %+v", de)
		}
		if fr.Translation != "" || fr.ErrorCode != suggestionTimeout {
			t.Errorf("fr: got %+v", fr)
		}
	})

	t.Run("sends the matching glossary terms", func(t *testing.T) {
		server, fake := newServer()
		if _, err := suggest(server, ctx, "st-entry", "st-es", "st-de"); err != nil {
			t.Fatalf("SuggestTranslations: %v", err)
		}
		es, _ := fake.request("Spanish")
		if es.Text != "Open the dashboard" {
			t.Errorf("es request: got %+v", es)
		}
		// Logo isn't in the text, and German has no glossary translation.
		if len(es.Terms) != 1 || es.Terms[0] != (translate.Term{Source: "Dashboard", Target: "Panel de control"}) {
			t.Errorf("es terms: got %+v", es.Terms)
		}
		if de, _ := fake.request("German"); len(de.Terms) != 0 {
			t.Errorf("de terms: got %+v", de.Terms)
		}
	})

	t.Run("reports failures per locale and keeps the others", func(t *testing.T) {
		server, fake := newServer()
		res, err := suggest(server, ctx, "st-entry", "st-sv", "st-fr", "st-de")
		if err != nil {
			t.Fatalf("SuggestTranslations: %v", err)
		}
		codes := []string{res.Suggestions[0].ErrorCode, res.Suggestions[1].ErrorCode, res.Suggestions[2].ErrorCode}
		if codes[0] != "UNSUPPORTED_LANGUAGE" || codes[1] != "FAILED" || codes[2] != "" {
			t.Errorf("error codes: got %v", codes)
		}
		if res.Suggestions[2].Translation != "Übersicht öffnen" {
			t.Errorf("de: got %+v", res.Suggestions[2])
		}
		// An unsupported language is never sent to the model.
		if _, sent := fake.request(""); sent {
			t.Errorf("sent a request without a language")
		}
	})

	t.Run("passes on warnings", func(t *testing.T) {
		db.Exec(`INSERT INTO localizations (id, entry_id, locale_id, translation) VALUES ('st-l3', 'st-empty', 'st-en', '{count} entries')`)
		server, fake := newServer()
		fake.answers["German"] = "{Anzahl} Einträge"
		res, err := suggest(server, ctx, "st-empty", "st-de")
		if err != nil {
			t.Fatalf("SuggestTranslations: %v", err)
		}
		warnings := res.Suggestions[0].Warnings
		if len(warnings) != 2 || warnings[0].Code != "PLACEHOLDER_MISSING" || warnings[0].Detail != "{count}" {
			t.Errorf("warnings: got %+v", warnings)
		}
		db.Exec(`DELETE FROM localizations WHERE id = 'st-l3'`)
	})

	t.Run("saves nothing", func(t *testing.T) {
		server, _ := newServer()
		if _, err := suggest(server, ctx, "st-entry", "st-es"); err != nil {
			t.Fatalf("SuggestTranslations: %v", err)
		}
		var count int64
		db.Table("localizations").Where("entry_id = 'st-entry' AND locale_id = 'st-es'").Count(&count)
		if count != 0 {
			t.Errorf("saved %d localizations", count)
		}
	})

	t.Run("rejects bad requests", func(t *testing.T) {
		server, _ := newServer()
		cases := []struct {
			name      string
			entryID   string
			localeIDs []string
			code      connect.Code
		}{
			{"missing entry", "", []string{"st-de"}, connect.CodeInvalidArgument},
			{"missing locales", "st-entry", nil, connect.CodeInvalidArgument},
			{"unknown entry", "st-unknown", []string{"st-de"}, connect.CodeNotFound},
			{"unknown locale", "st-entry", []string{"st-unknown"}, connect.CodeNotFound},
			{"no source text", "st-empty", []string{"st-de"}, connect.CodeFailedPrecondition},
		}
		for _, c := range cases {
			_, err := suggest(server, ctx, c.entryID, c.localeIDs...)
			if connect.CodeOf(err) != c.code {
				t.Errorf("%s: got %v, want %v", c.name, err, c.code)
			}
		}
	})

	t.Run("needs an enabled translation server", func(t *testing.T) {
		server := &Server{db: db, translators: fixedSource{}}
		_, err := suggest(server, ctx, "st-entry", "st-de")
		if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "not configured") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("needs permission to use machine translation", func(t *testing.T) {
		server, fake := newServer()
		editor, err := testutil.ContextWithPermissions(db, "read_entry", "update_entry")
		if err != nil {
			t.Fatalf("ContextWithPermissions: %v", err)
		}
		if _, err := suggest(server, editor, "st-entry", "st-de"); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("without use_machine_translation: got %v", err)
		}

		translator, err := testutil.ContextWithPermissions(db, "update_entry", "use_machine_translation")
		if err != nil {
			t.Fatalf("ContextWithPermissions: %v", err)
		}
		if _, err := suggest(server, translator, "st-entry", "st-de"); err != nil {
			t.Errorf("with both permissions: %v", err)
		}
		if len(fake.requests) != 1 {
			t.Errorf("sent %d requests, want 1", len(fake.requests))
		}
	})

	t.Run("needs permission to edit entries", func(t *testing.T) {
		server, fake := newServer()
		viewer := testutil.ContextWithTestUser(context.Background(), "st-nobody")
		_, err := suggest(server, viewer, "st-entry", "st-de")
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("got %v", err)
		}
		if len(fake.requests) != 0 {
			t.Errorf("sent %d requests", len(fake.requests))
		}
	})
}

func TestBatchTranslate(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}
	seedTranslationData(t, db)
	db.Exec(`INSERT INTO entries (id, key) VALUES ('st-warn', 'items.count')`)
	db.Exec(`INSERT INTO localizations (id, entry_id, locale_id, translation) VALUES ('st-l4', 'st-warn', 'st-en', '{count} items')`)

	fake := &fakeTranslator{answers: map[string]string{"German": "Übersicht öffnen (neu)", "Spanish": "Abrir"}}
	server := &Server{db: db, translators: fixedSource{fake}}

	t.Run("keeps existing translations and saves new ones", func(t *testing.T) {
		res, err := server.BatchTranslate(ctx, connect.NewRequest(&pb.BatchTranslateRequest{
			EntryIds:       []string{"st-entry", "st-warn"},
			TargetLocaleId: "st-es",
			SourceLocaleId: "st-en",
		}))
		if err != nil {
			t.Fatalf("BatchTranslate: %v", err)
		}
		// st-warn loses its placeholder, so it isn't saved.
		if res.Msg.SuccessCount != 1 || res.Msg.FailureCount != 1 || res.Msg.SkippedCount != 0 {
			t.Errorf("counts: got %+v", res.Msg)
		}
		var saved string
		db.Table("localizations").Select("translation").Where("entry_id = 'st-entry' AND locale_id = 'st-es'").Scan(&saved)
		if saved != "Abrir" {
			t.Errorf("saved: got %q", saved)
		}

		res, err = server.BatchTranslate(ctx, connect.NewRequest(&pb.BatchTranslateRequest{
			EntryIds:       []string{"st-entry"},
			TargetLocaleId: "st-de",
			SourceLocaleId: "st-en",
		}))
		if err != nil {
			t.Fatalf("BatchTranslate: %v", err)
		}
		if res.Msg.SkippedCount != 1 || res.Msg.SuccessCount != 0 {
			t.Errorf("counts: got %+v", res.Msg)
		}
		var kept string
		db.Table("localizations").Select("translation").Where("id = 'st-l2'").Scan(&kept)
		if kept != "Übersicht öffnen" {
			t.Errorf("existing translation changed to %q", kept)
		}
	})

	t.Run("needs permission to use machine translation", func(t *testing.T) {
		editor, err := testutil.ContextWithPermissions(db, "update_entry")
		if err != nil {
			t.Fatalf("ContextWithPermissions: %v", err)
		}
		_, err = server.BatchTranslate(editor, connect.NewRequest(&pb.BatchTranslateRequest{
			EntryIds: []string{"st-entry"}, TargetLocaleId: "st-fr", SourceLocaleId: "st-en",
		}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("got %v", err)
		}
	})

	t.Run("needs an enabled translation server", func(t *testing.T) {
		_, err := (&Server{db: db, translators: fixedSource{}}).BatchTranslate(ctx, connect.NewRequest(&pb.BatchTranslateRequest{
			EntryIds: []string{"st-entry"}, TargetLocaleId: "st-fr", SourceLocaleId: "st-en",
		}))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("got %v", err)
		}
	})
}
