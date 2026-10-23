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

		if answer != "Speichern" {
			t.Errorf("answer: got %q", answer)
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

		if answer != "{name} löschen" {
			t.Errorf("answer: got %q", answer)
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
