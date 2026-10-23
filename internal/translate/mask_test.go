package translate

import "testing"

func TestMask(t *testing.T) {
	tests := []struct {
		text, masked, answer, want string
	}{
		{"Save", "Save", "Speichern", "Speichern"},
		{"Delete {name}", "Delete {1}", "{1} löschen", "{name} löschen"},
		{
			"Hello {name}, you have {count} new messages",
			"Hello {1}, you have {2} new messages",
			"Hallo {1}, Sie haben {2} neue Nachrichten",
			"Hallo {name}, Sie haben {count} neue Nachrichten",
		},
		// Repeats share a number, and order may change in the translation.
		{"{count} of {count} by {user}", "{1} of {1} by {2}", "{2}: {1} von {1}", "{user}: {count} von {count}"},
		{"No items | One item | {count} items", "No items | One item | {1} items", "Keine | Eins | {1} Einträge", "Keine | Eins | {count} Einträge"},
		// Positional placeholders in the source are placeholders like any other.
		{"{1} and {name}", "{1} and {2}", "{1} und {2}", "{1} und {name}"},
		// Tokens the source doesn't have stay, so Clean can flag them.
		{"Delete {name}", "Delete {1}", "{1} {2} {01} löschen", "{name} {2} {01} löschen"},
	}
	for _, tt := range tests {
		masked, unmask := Mask(tt.text)
		if masked != tt.masked {
			t.Errorf("Mask(%q) = %q, want %q", tt.text, masked, tt.masked)
		}

		if got := unmask(tt.answer); got != tt.want {
			t.Errorf("unmask(%q) for %q = %q, want %q", tt.answer, tt.text, got, tt.want)
		}
	}
}
