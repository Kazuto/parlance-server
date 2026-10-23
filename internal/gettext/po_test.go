package gettext

import (
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	input := []Entry{
		{ID: "z.key", Translation: "Grüße \"world\"\nsecond\t\\line\x00\a", Description: "Context\nmore context"},
		{ID: "a.key"},
	}

	encoded := Marshal("de-DE", input)
	if !strings.Contains(encoded, `"Language: de-DE\n"`) || !strings.Contains(encoded, "#. Context\n#. more context") {
		t.Fatalf("missing metadata/comments: %s", encoded)
	}

	parsed, err := Parse([]byte(encoded))
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed) != 2 || parsed[0].ID != "a.key" || parsed[1].Translation != input[0].Translation {
		t.Fatalf("incorrect round trip: %+v", parsed)
	}

	if encoded != Marshal("de-DE", []Entry{input[1], input[0]}) {
		t.Fatal("export must be deterministic")
	}
}

func TestNumericEscapesAndTabs(t *testing.T) {
	entries, err := Parse([]byte("msgid\t\"key with space\"\nmsgstr \"\\303\\274\\x21\\n\"\n"))
	if err != nil || len(entries) != 1 || entries[0].Translation != "ü!\n" {
		t.Fatalf("numeric C escapes: %+v, %v", entries, err)
	}
}

func TestParseMultilineAndUnsupportedRecords(t *testing.T) {
	input := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "button."
"save"
msgstr "Spei"
"chern"

#, fuzzy, c-format
msgid "review"
msgstr "Review"

msgctxt "menu"
msgid "save"
msgstr "Save"

msgid "file"
msgid_plural "files"
msgstr[0] "Datei"
msgstr[1] "Dateien"

#~ msgid "old"
#~ msgstr "Alt"
`

	entries, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 5 || entries[0].ID != "button.save" || entries[0].Translation != "Speichern" || !entries[1].Fuzzy || !entries[2].Context || !entries[3].Plural || !entries[4].Obsolete {
		t.Fatalf("unexpected records: %+v", entries)
	}
}

func TestParseRejectsInvalidFiles(t *testing.T) {
	for _, input := range []string{
		"msgid \"key\"\n",
		"msgstr \"value\"\n",
		"msgid \"key\"\nmsgstr \"bad\\q\"\n",
		"msgid \"key\"\nmsgstr \"unterminated\n",
		"msgid \"key\"\nmsgstr \"value\"\nunknown \"value\"\n",
		"msgid \"\"\nmsgstr \"Content-Type: text/plain; charset=ISO-8859-1\\n\"\n",
		"msgid \"key\"\nmsgstr \"\xff\"\n",
	} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("expected error for %q", input)
		}
	}
}
