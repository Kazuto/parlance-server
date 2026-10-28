package translate

import "testing"

func TestWrongFormality(t *testing.T) {
	tests := []struct {
		name      string
		language  string
		formality Formality
		text      string
		want      string
	}{
		// The model's answer when asked for "du"
		{"formal German imperative", "German", Informal, "Geben Sie Ihren Namen ein.", "Sie"},
		{"formal German after a sentence-initial Sie", "German", Informal, "Sie können Ihr Passwort ändern.", "Ihr"},
		{"informal German", "German", Informal, "Du kannst dein Passwort ändern.", ""},
		// "Sie" starting a sentence is also "they".
		{"German they", "German", Informal, "Einträge gelöscht. Sie sind im Papierkorb.", ""},
		{"German they after a quote", "German", Informal, "„Sie sind gelöscht“", ""},
		{"German they after a colon", "German", Informal, "Hinweis: Sie sind gelöscht.", ""},
		{"German they on a new line", "German", Informal, "Gelöscht\nSie sind im Papierkorb.", ""},
		{"lowercase German she", "German", Informal, "Wenn sie es freigibt, ist es fertig.", ""},
		{"informal German, any case", "German", Formal, "Kopiere den Token. Danach siehst DU ihn nicht mehr.", "DU"},
		{"formal German", "German", Formal, "Sind Sie sicher, dass Sie den Eintrag löschen möchten?", ""},
		{"words within words", "German", Formal, "Duplikate sind verboten.", ""},
		{"placeholders", "German", Formal, "Hallo {du}, willkommen.", ""},
		{"formal French", "French", Informal, "Vous pouvez changer votre mot de passe.", "Vous"},
		{"informal French", "French", Formal, "Entre ton nom.", "ton"},
		{"informal Spanish", "Spanish", Formal, "Ingresa tu nombre.", "tu"},
		{"formal Spanish", "Spanish", Informal, "Usted puede cambiar su contraseña.", "Usted"},
		// "su" is also "his", "her" and "their", so it passes.
		{"Spanish su", "Spanish", Informal, "Puede cambiar su contraseña.", ""},
		{"formal Italian", "Italian", Informal, "Grazie per la Sua pazienza.", "Sua"},
		{"informal Italian", "Italian", Formal, "Inserisci il tuo nome.", "tuo"},
		{"formal Dutch", "Dutch", Informal, "Vul uw naam in.", "uw"},
		{"informal Dutch", "Dutch", Formal, "Weet je het zeker?", "je"},
		{"formal Russian", "Russian", Informal, "Вы уверены?", "Вы"},
		{"informal Russian", "Russian", Formal, "Ты точно хочешь удалить эту запись?", "Ты"},
		{"a language without known forms", "Japanese", Formal, "削除してもよろしいですか？", ""},
		{"no formality", "German", "", "Du kannst Sie fragen.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WrongFormality(tt.language, tt.formality, tt.text); got != tt.want {
				t.Errorf("WrongFormality(%q, %q, %q) = %q, want %q", tt.language, tt.formality, tt.text, got, tt.want)
			}
		})
	}
}
