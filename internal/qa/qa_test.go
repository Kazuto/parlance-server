package qa

import (
	"slices"
	"testing"

	"github.com/kazuto/parlance-server/internal/translate"
)

// These are the cases of the web app's test/nuxt/qa.spec.ts: both must agree.
func codes(source, translation string, target Target) []string {
	out := []string{}

	for _, warning := range Check(source, translation, target) {
		if warning.Detail != "" {
			out = append(out, warning.Code+" "+warning.Detail)
		} else {
			out = append(out, warning.Code)
		}
	}

	return out
}

func TestCheck(t *testing.T) {
	tests := []struct {
		source, translation string
		want                []string
	}{
		{"Hello {name}", "Hallo {name}", nil},
		{"{count} items", "{Anzahl} Einträge", []string{"PLACEHOLDER_MISSING {count}", "PLACEHOLDER_UNEXPECTED {Anzahl}"}},
		{"One item | {count} items", "{count} Einträge", []string{"PLURAL_FORMS"}},
		{"Save", "Speichern\nSichern", []string{"LINE_BREAKS"}},
		{"%s of %1$d files", "%s von Dateien", []string{"PLACEHOLDER_MISSING %1$d"}},
		{"Welcome, :name", "Willkommen, :Name", []string{"PLACEHOLDER_MISSING :name", "PLACEHOLDER_UNEXPECTED :Name"}},
		{"Opens at 10:30", "Öffnet um 10:30", nil},
		{"See https://parlance.dev", "Siehe https://parlance.dev", nil},
		{"100% done", "100 % fertig", nil},
		{"50%% off", "50 %% Rabatt", nil},
		{"Note: read this", "Hinweis: lies das", nil},
		{" Save", "Speichern", []string{"WHITESPACE_START"}},
		{"Save ", "Speichern", []string{"WHITESPACE_END"}},
		{"Save", "Speichern\n", []string{"WHITESPACE_END"}},
		{"<b>Save</b> now", "<b>Jetzt</b> speichern", nil},
		{`<a href="/help">Help</a>`, `<a href="/hilfe">Hilfe</a>`, nil},
		{"<b>Save</b> now", "Jetzt speichern</b>", []string{"TAG_MISSING <b>"}},
		{"Line<br>break", "Zeilen<br/>umbruch<br>", []string{"TAG_UNEXPECTED <br>"}},
		{"a < b > c", "a < b > c", nil},
		{"Delete?", "Löschen", []string{"PUNCTUATION_END ?"}},
		{"Saved", "Gespeichert.", []string{"PUNCTUATION_END"}},
		{"Saved.", "保存しました。", nil},
		{"Loading...", "Laden…", nil},
		{"Delete?", "Διαγραφή;", nil},
		{"Hello {name}", "", nil},
		{"  ", "Hallo", nil},
	}

	for _, test := range tests {
		got := codes(test.source, test.translation, Target{})
		if !slices.Equal(got, append([]string{}, test.want...)) {
			t.Errorf("Check(%q, %q) = %v, want %v", test.source, test.translation, got, test.want)
		}
	}
}

func TestCheckFormality(t *testing.T) {
	formal, informal := translate.Formal, translate.Informal

	// The sources match the translations in everything but the form of address.
	tests := []struct {
		code                string
		formality           translate.Formality
		source, translation string
		want                []string
	}{
		{"de", informal, "Enter your name.", "Geben Sie Ihren Namen ein.", []string{"FORMALITY Sie"}},
		{"de", informal, "Entries deleted. They are in the bin.", "Einträge gelöscht. Sie sind im Papierkorb.", nil},
		{"de", formal, "Copy the token. You won't see it again.", "Kopiere den Token. Danach siehst DU ihn nicht mehr.", []string{"FORMALITY DU"}},
		{"de", formal, "Hello {du}, welcome.", "Hallo {du}, willkommen.", nil},
		{"fr", informal, "You can change your password.", "Vous pouvez changer votre mot de passe.", []string{"FORMALITY Vous"}},
		{"es", informal, "You can change your password.", "Puede cambiar su contraseña.", nil},
		{"de-AT", formal, "You can change it.", "Du kannst es ändern.", []string{"FORMALITY Du"}},
		{"pt_BR", formal, "You can change it.", "Você pode mudar.", nil},
		{"de", "", "You can ask them.", "Du kannst Sie fragen.", nil},
	}

	for _, test := range tests {
		got := codes(test.source, test.translation, Target{Code: test.code, Formality: test.formality})

		if !slices.Equal(got, append([]string{}, test.want...)) {
			t.Errorf("%s %s: Check(%q) = %v, want %v", test.code, test.formality, test.translation, got, test.want)
		}
	}
}
