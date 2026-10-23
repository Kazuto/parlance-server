package translate

import (
	"errors"
	"reflect"
	"testing"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		answer   string
		want     string
		warnings []Warning
	}{
		{name: "trims whitespace", source: "Save", answer: "  Speichern\n", want: "Speichern"},
		{name: "removes a code block", source: "Save", answer: "```\nSpeichern\n```", want: "Speichern"},
		{name: "removes a code block with a language", source: "Save", answer: "```text\nSpeichern\n```", want: "Speichern"},
		{name: "removes quotes", source: "Save", answer: `"Speichern"`, want: "Speichern"},
		{name: "removes typographic quotes", source: "Save", answer: "„Speichern“", want: "Speichern"},
		{name: "keeps quotes the source has", source: `"Save"`, answer: `"Speichern"`, want: `"Speichern"`},
		{name: "keeps quotes inside the text", source: `Click "Save"`, answer: `Klicke auf "Speichern"`, want: `Klicke auf "Speichern"`},
		{name: "keeps placeholders", source: "Hello {name}", answer: "Hallo {name}", want: "Hallo {name}"},
		{
			name: "warns about a translated placeholder", source: "{count} items", answer: "{Anzahl} Einträge",
			want: "{Anzahl} Einträge",
			warnings: []Warning{
				{Code: WarningPlaceholderMissing, Detail: "{count}"},
				{Code: WarningPlaceholderUnexpected, Detail: "{Anzahl}"},
			},
		},
		{
			name: "warns about lost plural forms", source: "One item | {count} items", answer: "{count} Einträge",
			want: "{count} Einträge", warnings: []Warning{{Code: WarningPluralForms}},
		},
		{
			name: "warns about added line breaks", source: "Save", answer: "Speichern\nSichern",
			want: "Speichern\nSichern", warnings: []Warning{{Code: WarningLineBreaks}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings, err := Clean(tt.source, tt.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("text: got %q, want %q", got, tt.want)
			}

			if !reflect.DeepEqual(warnings, tt.warnings) {
				t.Errorf("warnings: got %v, want %v", warnings, tt.warnings)
			}
		})
	}

	t.Run("rejects an empty answer", func(t *testing.T) {
		for _, answer := range []string{"", "  \n", `""`, "```\n```"} {
			if _, _, err := Clean("Save", answer); !errors.Is(err, ErrEmptyAnswer) {
				t.Errorf("%q: got %v, want ErrEmptyAnswer", answer, err)
			}
		}
	})

	t.Run("rejects an answer that explains instead of translating", func(t *testing.T) {
		answer := "Here is the translation of the word you sent me. In German, the word is commonly translated as follows, depending on context: Speichern."
		if _, _, err := Clean("Save", answer); !errors.Is(err, ErrAnswerTooLong) {
			t.Errorf("got %v, want ErrAnswerTooLong", err)
		}
	})
}
