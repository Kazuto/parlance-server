package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	t.Run("sends the prompt and returns the answer", func(t *testing.T) {
		var got chatRequest

		var auth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" {
				t.Errorf("path: got %q", r.URL.Path)
			}

			auth = r.Header.Get("Authorization")

			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode: %v", err)
			}

			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Speichern"}}]}`))
		}))
		defer server.Close()

		client := NewClient(server.URL+"/v1/", "hy-mt2", "secret", time.Second)

		answer, err := client.Translate(context.Background(), Request{Text: "Save", TargetLang: "German"})
		if err != nil {
			t.Fatalf("Translate: %v", err)
		}

		// Without a model in the answer, the configured one wrote it.
		if answer.Text != "Speichern" || answer.Model != "hy-mt2" {
			t.Errorf("answer: got %+v", answer)
		}

		if got.Model != "hy-mt2" || len(got.Messages) != 1 || got.Messages[0].Role != "user" {
			t.Errorf("request: got %+v", got)
		}

		if got.Messages[0].Content != Prompt(Request{Text: "Save", TargetLang: "German"}) {
			t.Errorf("prompt: got %q", got.Messages[0].Content)
		}

		if auth != "Bearer secret" {
			t.Errorf("authorization: got %q", auth)
		}
	})

	t.Run("masks placeholders and restores them in the answer", func(t *testing.T) {
		var got chatRequest

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode: %v", err)
			}

			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{1} löschen"}}]}`))
		}))
		defer server.Close()

		answer, err := NewClient(server.URL, "", "", time.Second).Translate(context.Background(), Request{Text: "Delete {name}", TargetLang: "German"})
		if err != nil {
			t.Fatalf("Translate: %v", err)
		}

		if !strings.HasSuffix(got.Messages[0].Content, "\n\nDelete {1}") {
			t.Errorf("prompt: got %q", got.Messages[0].Content)
		}

		// The context names the placeholders as the text does
		_, err = NewClient(server.URL, "", "", time.Second).Translate(context.Background(), Request{
			Text: "Delete {name}", TargetLang: "German", Context: "{name} is the scope's name.",
		})
		if err != nil {
			t.Fatalf("Translate: %v", err)
		}

		if content := got.Messages[0].Content; !strings.Contains(content, "[Background Information]\n{1} is the scope's name.") ||
			!strings.HasSuffix(content, "[Source Text]\nDelete {1}") {
			t.Errorf("prompt with context: got %q", content)
		}

		if answer.Text != "{name} löschen" {
			t.Errorf("answer: got %q", answer.Text)
		}
	})

	t.Run("sends no key when none is set", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Errorf("unexpected authorization header")
			}

			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Speichern"}}]}`))
		}))
		defer server.Close()

		if _, err := NewClient(server.URL, "", "", time.Second).Translate(context.Background(), Request{Text: "Save"}); err != nil {
			t.Fatalf("Translate: %v", err)
		}
	})

	t.Run("reports an error answer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "model not loaded", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		_, err := NewClient(server.URL, "", "", time.Second).Translate(context.Background(), Request{Text: "Save"})
		if err == nil || !strings.Contains(err.Error(), "model not loaded") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("reports an answer without choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"choices":[]}`))
		}))
		defer server.Close()

		_, err := NewClient(server.URL, "", "", time.Second).Translate(context.Background(), Request{Text: "Save"})
		if err != ErrEmptyAnswer {
			t.Fatalf("got %v, want ErrEmptyAnswer", err)
		}
	})
}

func TestClientModels(t *testing.T) {
	t.Run("names the model that answered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"model":"tencent/Hy-MT2-7B","choices":[{"message":{"content":"Speichern"}}]}`))
		}))
		defer server.Close()

		answer, err := NewClient(server.URL, "", "", time.Second).Translate(context.Background(), Request{Text: "Save"})
		if err != nil || answer.Model != "tencent/Hy-MT2-7B" {
			t.Fatalf("got %+v, %v", answer, err)
		}
	})

	t.Run("lists the served models and the configured one", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/models" {
				http.NotFound(w, r)

				return
			}

			_, _ = w.Write([]byte(`{"data":[{"id":"tencent/Hy-MT2-7B"},{"id":"hy-mt2"}]}`))
		}))
		defer server.Close()

		models, err := NewClient(server.URL+"/v1", "hy-mt2", "", time.Second).Models(context.Background())
		if err != nil || strings.Join(models, ",") != "hy-mt2,tencent/Hy-MT2-7B" {
			t.Fatalf("got %v, %v", models, err)
		}
	})

	t.Run("reports a server that can't list its models", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusInternalServerError)
		}))
		defer server.Close()

		if _, err := NewClient(server.URL, "", "", time.Second).Models(context.Background()); err == nil {
			t.Fatal("got no error")
		}
	})
}
