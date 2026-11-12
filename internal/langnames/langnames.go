// Package langnames names the parts of locale codes such as "pt-BR", from the
// CLDR data in golang.org/x/text.
package langnames

import (
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

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
