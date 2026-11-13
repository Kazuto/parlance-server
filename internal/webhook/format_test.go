package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

// payloadOf renders an event as Emit stores it.
func payloadOf(t *testing.T, event string, data any) string {
	t.Helper()

	payload, err := render("event-1", event, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	return payload
}

func discordBody(t *testing.T, payload string) map[string]any {
	t.Helper()

	body, err := Body(models.WebhookFormatDiscord, payload)
	if err != nil {
		t.Fatalf("Body() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}

	return decoded
}

func slackText(t *testing.T, payload string) string {
	t.Helper()

	body, err := Body(models.WebhookFormatSlack, payload)
	if err != nil {
		t.Fatalf("Body() error = %v", err)
	}

	var decoded struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}

	return decoded.Text
}

func TestJSONBodyIsThePayload(t *testing.T) {
	payload := payloadOf(t, models.WebhookEventEntriesCreated, EntriesData{Count: 1})

	for _, format := range []string{models.WebhookFormatJSON, ""} {
		if body, err := Body(format, payload); err != nil || string(body) != payload {
			t.Errorf("format %q: body = %s, %v", format, body, err)
		}
	}
}

func TestDiscordMessages(t *testing.T) {
	translations := make([]Translation, 12)
	for i := range translations {
		translations[i] = Translation{Key: fmt.Sprintf("key.%02d", i), Locale: "de", Translation: "@everyone Speichern\nnow"}
	}

	approved := discordBody(t, payloadOf(t, models.WebhookEventTranslationsApproved, TranslationsData{
		Count: 12, Translations: translations, User: &User{Name: "Rita"},
	}))

	content, _ := approved["content"].(string)
	if !strings.HasPrefix(content, "Rita approved 12 translations\n• `key.00` (de): @everyone Speichern now") ||
		!strings.Contains(content, "`key.09`") || strings.Contains(content, "`key.10`") ||
		!strings.HasSuffix(content, "…and 2 more") {
		t.Errorf("content = %q", content)
	}

	// The texts may say @everyone, but nobody is pinged.
	if mentions, _ := approved["allowed_mentions"].(map[string]any); mentions == nil || fmt.Sprint(mentions["parse"]) != "[]" {
		t.Errorf("allowed_mentions = %v", approved["allowed_mentions"])
	}

	if approved["username"] != "Parlance" {
		t.Errorf("username = %v", approved["username"])
	}

	long := discordBody(t, payloadOf(t, models.WebhookEventTranslationsRejected, TranslationsData{
		Count: 1, Comment: strings.Repeat("too long ", 400), User: &User{Name: "Rita"},
	}))

	if content, _ := long["content"].(string); len([]rune(content)) != discordMaxContent || !strings.HasSuffix(content, "…") {
		t.Errorf("long message has %d characters, want it cut at %d", len([]rune(content)), discordMaxContent)
	}
}

func TestChatMessages(t *testing.T) {
	tests := []struct {
		event string
		data  any
		want  string
	}{
		{
			models.WebhookEventTranslationsRejected,
			TranslationsData{Count: 1, Comment: "Use the <b> tag", Translations: []Translation{{Key: "a`b", Locale: "fr", Translation: "Enregistrer"}}, User: &User{Name: "Rita"}},
			"Rita rejected 1 translation: “Use the &lt;b&gt; tag”\n• `a'b` (fr): Enregistrer",
		},
		{
			models.WebhookEventEntriesCreated,
			EntriesData{Count: 2, Entries: []Entry{{Key: "one"}, {Key: "two"}}},
			"2 entries added\n• `one`\n• `two`",
		},
		{
			models.WebhookEventTaskRunSucceeded,
			TaskRunData{Task: TaskInfo{Name: "Ship"}, Run: RunInfo{Status: models.RunSucceeded, Summary: models.TaskRunSummary{PullRequestURL: "https://github.example/pr/7"}}},
			"Task “Ship” succeeded\nPull request: https://github.example/pr/7",
		},
		{
			models.WebhookEventTaskRunSucceeded,
			TaskRunData{Task: TaskInfo{Name: "Fill"}, Run: RunInfo{Status: models.RunSucceeded, Summary: models.TaskRunSummary{Translated: 5, Failed: 1}}},
			"Task “Fill” succeeded: 5 translated, 1 failed",
		},
		{
			models.WebhookEventTaskRunFailed,
			TaskRunData{Task: TaskInfo{Name: "Ship"}, Run: RunInfo{Status: models.RunFailed, Error: "the token can't push & pull"}},
			"Task “Ship” failed: the token can't push &amp; pull",
		},
		{
			models.WebhookEventTranslationsReleased,
			ReleaseData{Release: ReleaseInfo{Name: "Spring"}, Count: 2, Entries: []Entry{{Key: "one"}, {Key: "two"}}, User: &User{Name: "Rita"}},
			"Rita released “Spring”: 2 entries export now\n• `one`\n• `two`",
		},
		{
			models.WebhookEventTranslationsReleased,
			ReleaseData{Release: ReleaseInfo{Name: "Spring"}, Count: 1, Entries: []Entry{{Key: "one"}}},
			"Release “Spring” went out: 1 entry exports now\n• `one`",
		},
		{
			models.WebhookEventTranslationsReleased,
			ReleaseData{Release: ReleaseInfo{Name: "Empty"}, Entries: []Entry{}},
			"Release “Empty” went out",
		},
		{
			models.WebhookEventExportFailed,
			ExportFailedData{Source: ExportSourceTask, Task: &TaskInfo{Name: "Ship"}, Error: "the token can't push\nto main"},
			"Git export “Ship” failed: the token can't push to main",
		},
		{
			models.WebhookEventExportFailed,
			ExportFailedData{Source: ExportSourceDownload, Format: "po", Locales: []string{"de"}, User: &User{Name: "Rita"}, Error: "failed to query entries"},
			"Rita’s download of de as PO failed: failed to query entries",
		},
		{
			models.WebhookEventExportFailed,
			ExportFailedData{Source: ExportSourceDownload, Format: "zip", Error: "failed to close ZIP archive"},
			"A ZIP download failed: failed to close ZIP archive",
		},
		{
			models.WebhookEventLoginFailed,
			LoginFailedData{Email: "rita@example.com", Method: LoginMethodPassword, Reason: LoginReasonWrongPassword, IP: "198.51.100.1", Attempts: 4},
			"Failed sign-in as rita@example.com from 198.51.100.1: wrong password (4 attempts)",
		},
		{
			models.WebhookEventLoginFailed,
			LoginFailedData{Email: "tom@example.com", Method: LoginMethodSSO, Provider: &ProviderInfo{ID: "p1", Name: "Keycloak"}, Reason: "sso_no_account", IP: "2001:db8::1", Attempts: 1},
			"Failed sign-in as tom@example.com with Keycloak from 2001:db8::1: no account has this email",
		},
		{
			models.WebhookEventLoginFailed,
			LoginFailedData{Method: LoginMethodSSO, Provider: &ProviderInfo{ID: "p1"}, Reason: "sso_something_new", Attempts: 1},
			"Failed sign-in: sso_something_new",
		},
	}

	for _, test := range tests {
		if got := slackText(t, payloadOf(t, test.event, test.data)); got != test.want {
			t.Errorf("%s:\n got %q\nwant %q", test.event, got, test.want)
		}
	}

	hook := models.Webhook{ID: "hook-1", Name: "Discord", Events: []string{models.WebhookEventEntriesCreated}}

	ping, err := Ping(&hook)
	if err != nil {
		t.Fatal(err)
	}

	if got := slackText(t, ping.Payload); got != "This is a test message from the Parlance webhook “Discord”." {
		t.Errorf("ping = %q", got)
	}

	if _, err := Body(models.WebhookFormatSlack, "not json"); err == nil {
		t.Error("an unreadable payload rendered")
	}
}

func TestAppriseBody(t *testing.T) {
	apprise := func(t *testing.T, payload string) appriseMessage {
		t.Helper()

		body, err := Body(models.WebhookFormatApprise, payload)
		if err != nil {
			t.Fatalf("Body() error = %v", err)
		}

		var decoded appriseMessage
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("body %s: %v", body, err)
		}

		return decoded
	}

	t.Run("titles the chat text, unescaped", func(t *testing.T) {
		got := apprise(t, payloadOf(t, models.WebhookEventTranslationsRejected, TranslationsData{
			Count: 1, Comment: "Use the <b> tag", User: &User{Name: "Rita"},
			Translations: []Translation{{Key: "save", Locale: "fr", Translation: "Enregistrer"}},
		}))
		want := appriseMessage{
			Title:  "Parlance: Translations rejected",
			Body:   "Rita rejected 1 translation: “Use the <b> tag”\n• `save` (fr): Enregistrer",
			Type:   "warning",
			Format: "text",
		}

		if got != want {
			t.Errorf("got %+v\nwant %+v", got, want)
		}
	})

	t.Run("marks how each event went", func(t *testing.T) {
		tests := []struct {
			event       string
			data        any
			title, kind string
		}{
			{models.WebhookEventTaskRunFailed, TaskRunData{Run: RunInfo{Status: models.RunFailed}}, "Task failed", "failure"},
			{models.WebhookEventTaskRunSucceeded, TaskRunData{Run: RunInfo{Status: models.RunSucceeded}}, "Task succeeded", "success"},
			{models.WebhookEventExportFailed, ExportFailedData{Source: ExportSourceDownload, Format: "json"}, "Export failed", "failure"},
			{models.WebhookEventLoginFailed, LoginFailedData{Email: "rita@example.com"}, "Sign-in failed", "warning"},
			{models.WebhookEventTranslationsApproved, TranslationsData{Count: 1}, "Translations approved", "success"},
			{models.WebhookEventTranslationsReleased, ReleaseData{Release: ReleaseInfo{Name: "Spring"}}, "Release went out", "success"},
			{models.WebhookEventTranslationsAwaitingApproval, TranslationsData{Count: 1}, "Translations await approval", "info"},
			{models.WebhookEventEntriesCreated, EntriesData{Count: 1}, "Entries added", "info"},
			{models.WebhookEventDigest, DigestData{Since: time.Now()}, "Digest", "info"},
			{"something.new", struct{}{}, "something.new", "info"},
		}

		for _, tt := range tests {
			got := apprise(t, payloadOf(t, tt.event, tt.data))
			if got.Title != "Parlance: "+tt.title || got.Type != tt.kind {
				t.Errorf("%s: got %q as %q, want %q as %q", tt.event, got.Title, got.Type, "Parlance: "+tt.title, tt.kind)
			}
		}
	})

	t.Run("sends a test message", func(t *testing.T) {
		ping, err := Ping(&models.Webhook{ID: "hook-1", Name: "Alerts"})
		if err != nil {
			t.Fatal(err)
		}

		got := apprise(t, ping.Payload)
		if got.Title != "Parlance: Test message" || got.Body != "This is a test message from the Parlance webhook “Alerts”." {
			t.Errorf("ping = %+v", got)
		}
	})
}

func TestAwaitingApprovalMessages(t *testing.T) {
	tests := []struct {
		data TranslationsData
		want string
	}{
		{
			TranslationsData{Count: 2, User: &User{Name: "Rita"}, Translations: []Translation{{Key: "greeting", Locale: "de", Translation: "Hallo"}}},
			"Rita submitted 2 translations for approval\n• `greeting` (de): Hallo\n…and 1 more",
		},
		{
			TranslationsData{Count: 1, Task: &TaskInfo{Name: "Nightly drafts"}, Translations: []Translation{{Key: "bye", Locale: "fr", Translation: "Au revoir"}}},
			"Task “Nightly drafts” drafted 1 translation for approval\n• `bye` (fr): Au revoir",
		},
		{
			TranslationsData{Count: 1, Translations: []Translation{{Key: "bye", Locale: "es", Translation: "Adiós"}}},
			"1 translation awaits approval\n• `bye` (es): Adiós",
		},
	}

	for _, test := range tests {
		if got := slackText(t, payloadOf(t, models.WebhookEventTranslationsAwaitingApproval, test.data)); got != test.want {
			t.Errorf("got %q\nwant %q", got, test.want)
		}
	}
}

func TestDigestMessage(t *testing.T) {
	raw := func(event string, data any) json.RawMessage {
		return json.RawMessage(payloadOf(t, event, data))
	}
	translations := func(keys ...string) []Translation {
		list := make([]Translation, len(keys))
		for i, key := range keys {
			list[i] = Translation{Key: key, Locale: "de"}
		}

		return list
	}

	data := DigestData{
		Since: time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC),
		Count: 9,
		Events: []json.RawMessage{
			raw(models.WebhookEventTranslationsAwaitingApproval, TranslationsData{Count: 2, User: &User{Name: "Rita"}, Translations: translations("a", "b")}),
			raw(models.WebhookEventTranslationsAwaitingApproval, TranslationsData{Count: 6, Task: &TaskInfo{Name: "Nightly"}, Translations: translations("c", "d", "e", "f", "g", "h")}),
			raw(models.WebhookEventTranslationsApproved, TranslationsData{Count: 1, User: &User{Name: "Tom"}, Translations: translations("a")}),
			raw(models.WebhookEventEntriesCreated, EntriesData{Count: 2, User: &User{Name: "Rita"}, Entries: []Entry{{Key: "x"}, {Key: "y"}}}),
			raw(models.WebhookEventTaskRunFailed, TaskRunData{Task: TaskInfo{Name: "Ship"}, Run: RunInfo{Status: models.RunFailed, Error: "boom"}}),
			raw(models.WebhookEventTranslationsReleased, ReleaseData{Release: ReleaseInfo{Name: "Spring"}, Count: 3, Entries: []Entry{{Key: "x"}}}),
			raw(models.WebhookEventTaskRunSucceeded, TaskRunData{Task: TaskInfo{Name: "Fill"}, Run: RunInfo{Status: models.RunSucceeded, Summary: models.TaskRunSummary{PullRequestURL: "https://github.example/pr/1"}}}),
			raw(models.WebhookEventLoginFailed, LoginFailedData{Email: "rita@example.com", Reason: LoginReasonUnknownEmail, IP: "198.51.100.1", Attempts: 2}),
		},
		Truncated: true,
	}

	want := "Parlance digest since 23 Oct 09:00 UTC\n" +
		"• 8 translations submitted for approval (Rita, Task “Nightly”): `a`, `b`, `c`, `d`, `e`, …\n" +
		"• 1 translation approved (Tom): `a`\n" +
		"• 2 entries added (Rita): `x`, `y`\n" +
		"• Release “Spring” went out: 3 entries export now\n" +
		"• Task “Ship” failed: boom\n" +
		"• Task “Fill” succeeded · Pull request: https://github.example/pr/1\n" +
		"• Failed sign-in as rita@example.com from 198.51.100.1: no account has this email (2 attempts)\n" +
		"…and 1 more event"

	if got := slackText(t, payloadOf(t, models.WebhookEventDigest, data)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
