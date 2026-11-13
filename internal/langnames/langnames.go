// Package langnames names the parts of locale codes such as "pt-BR", from the
// CLDR data in golang.org/x/text.
package langnames

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// maxCodeLength is the longest code a locale takes.
const maxCodeLength = 10

// Valid tells whether code is a language code that fits a locale, e.g. "sv",
// "pt-BR" or "pt_BR". Made-up codes such as "en-informal" aren't.
func Valid(code string) bool {
	_, ok := parse(code)

	return ok && len(code) <= maxCodeLength
}

// Name returns the name of the language with the code in the language in,
// capitalized as in a list, e.g. "Schwedisch" for "sv" in "de", or "" when either
// is unknown.
func Name(code, in string) string {
	tag, ok := parse(code)
	if !ok {
		return ""
	}

	inTag, ok := parse(in)
	if !ok {
		return ""
	}

	namer := display.Tags(inTag)
	if namer == nil {
		return ""
	}

	name := namer.Name(tag)
	if name == "" {
		return ""
	}

	first, size := utf8.DecodeRuneInString(name)

	return string(unicode.ToUpper(first)) + name[size:]
}

// Names returns the names of the language with the code in each of the languages
// it knows, by their codes as given.
func Names(code string, ins []string) map[string]string {
	names := map[string]string{}

	for _, in := range ins {
		if name := Name(code, in); name != "" {
			names[in] = name
		}
	}

	return names
}

// parse reads a code with "-" or "_" between its parts, which must name a known
// language.
func parse(code string) (language.Tag, bool) {
	tag, err := language.Parse(strings.ReplaceAll(code, "_", "-"))
	if err != nil {
		return language.Und, false
	}

	if _, confidence := tag.Base(); confidence != language.Exact {
		return language.Und, false
	}

	return tag, true
}

// Region returns the region a code names, by its code and English name, e.g.
// "BR" and "Brazil" for "pt-BR", or "419" and "Latin America" for "es_419". ok
// is false when the code names none: "de" alone doesn't, though x/text would
// guess Germany.
func Region(code string) (id, name string, ok bool) {
	tag, err := language.Parse(strings.ReplaceAll(code, "_", "-"))
	if err != nil {
		return "", "", false
	}

	region, confidence := tag.Region()
	if confidence != language.Exact {
		return "", "", false
	}

	return region.String(), display.English.Regions().Name(region), true
}
