package translate

import "testing"

func TestPrompt(t *testing.T) {
	t.Run("asks for user interface text", func(t *testing.T) {
		got := Prompt(Request{Text: "Save changes", TargetLang: "German"})
		want := "Please translate the following text into German. Note that the translation style must strictly conform to [software user interface text]:\n\nSave changes"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
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
			"Please translate the following text into Spanish. Note that the translation style must strictly conform to [software user interface text]:\n\nOpen the dashboard"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
