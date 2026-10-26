package translate

import (
	"strings"
	"testing"
)

func TestPrompt(t *testing.T) {
	t.Run("asks for user interface text", func(t *testing.T) {
		got := Prompt(Request{Text: "Save changes", TargetLang: "German"})
		want := "Please translate the following text into German. Note that the translation style must strictly conform to [software user interface text]. Only output the translated result without any additional explanation:\n\nSave changes"

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("asks for the form of address by its pronoun", func(t *testing.T) {
		tests := []struct {
			req  Request
			want string
		}{
			{Request{TargetLang: "German", Formality: Formal}, "[software user interface text, formal, addressing the user as Sie]"},
			{Request{TargetLang: "French", Formality: Informal}, "[software user interface text, informal, addressing the user as tu]"},
			// Without a known pronoun, in words
			{Request{TargetLang: "Japanese", Formality: Formal}, "[software user interface text, formal, addressing the user politely]"},
			{Request{TargetLang: "Japanese", Formality: Informal}, "[software user interface text, informal, addressing the user casually]"},
			{Request{TargetLang: "German", Formality: "casual"}, "[software user interface text]"},
		}

		for _, tt := range tests {
			tt.req.Text = "Save changes"
			if got := Prompt(tt.req); !strings.Contains(got, tt.want) {
				t.Errorf("Prompt(%+v) = %q, want it to contain %q", tt.req, got, tt.want)
			}
		}
	})

	t.Run("lists glossary terms before the text", func(t *testing.T) {
		got := Prompt(Request{
			Text:       "Open the dashboard",
			TargetLang: "Spanish",
			Terms: []Term{
				{Source: "Dashboard", Target: "Panel de control"},
				{Source: "Entry", Target: "Entrada"},
			},
		})
		want := "Reference the following translations:\n" +
			"Dashboard translates to Panel de control\n" +
			"Entry translates to Entrada\n\n" +
			"Please translate the following text into Spanish. Note that the translation style must strictly conform to [software user interface text]. Only output the translated result without any additional explanation:\n\nOpen the dashboard"

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
