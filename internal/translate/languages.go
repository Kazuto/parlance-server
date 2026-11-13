package translate

import (
	"fmt"
	"strings"

	"github.com/kazuto/parlance-server/internal/langnames"
)

// languageNames maps language subtags to the names Hy-MT2 knows.
var languageNames = map[string]string{
	"ar": "Arabic", "bn": "Bengali", "bo": "Tibetan", "cs": "Czech",
	"de": "German", "en": "English", "es": "Spanish", "fa": "Persian",
	"fil": "Filipino", "fr": "French", "gu": "Gujarati", "he": "Hebrew",
	"hi": "Hindi", "id": "Indonesian", "it": "Italian", "iw": "Hebrew",
	"ja": "Japanese", "kk": "Kazakh", "km": "Khmer", "ko": "Korean",
	"mn": "Mongolian", "mr": "Marathi", "ms": "Malay", "my": "Burmese",
	"nl": "Dutch", "pl": "Polish", "pt": "Portuguese", "ru": "Russian",
	"ta": "Tamil", "te": "Telugu", "th": "Thai", "tl": "Filipino",
	"tr": "Turkish", "ug": "Uyghur", "uk": "Ukrainian", "ur": "Urdu",
	"vi": "Vietnamese", "yue": "Cantonese", "zh": "Chinese",
}

// variantNames are the names regional variants commonly go by in English, by
// language and region, rather than e.g. "Portuguese (Brazil)".
var variantNames = map[string]string{
	"en-US": "American English", "en-GB": "British English",
	"en-AU": "Australian English", "en-CA": "Canadian English",
	"pt-BR": "Brazilian Portuguese", "pt-PT": "European Portuguese",
	"es-ES": "European Spanish", "es-MX": "Mexican Spanish",
	"es-419": "Latin American Spanish",
	"fr-CA":  "Canadian French", "fr-CH": "Swiss French",
	"de-AT": "Austrian German", "de-CH": "Swiss Standard German",
	"nl-BE": "Flemish",
}

// LanguageName returns the model's name for a locale code such as "de" or
// "pt-BR", and false when the models don't support the language.
func LanguageName(code string) (string, bool) {
	parts := codeParts(code)
	if len(parts) == 0 {
		return "", false
	}

	if parts[0] == "zh" {
		for _, part := range parts[1:] {
			// Taiwan, Hong Kong and Macau write in traditional characters.
			if part == "hant" || part == "tw" || part == "hk" || part == "mo" {
				return "Traditional Chinese", true
			}
		}
	}

	name, ok := languageNames[parts[0]]

	return name, ok
}

// TargetName names the language a locale code asks for, with its region: e.g.
// "Brazilian Portuguese" for "pt-BR", "Italian (Switzerland)" for "it-CH", and
// LanguageName for a code without one. Chinese and Cantonese keep their
// LanguageName, which already tells the scripts apart. It returns "" when the
// models don't support the language.
func TargetName(code string) string {
	name, ok := LanguageName(code)
	if !ok {
		return ""
	}

	base := codeParts(code)[0]
	if base == "zh" || base == "yue" {
		return name
	}

	region, regionName, ok := langnames.Region(code)
	if !ok {
		return name
	}

	if variant, ok := variantNames[base+"-"+region]; ok {
		return variant
	}

	return fmt.Sprintf("%s (%s)", name, regionName)
}

// codeParts splits a locale code into its lowercase subtags.
func codeParts(code string) []string {
	return strings.FieldsFunc(strings.ToLower(code), func(r rune) bool {
		return r == '-' || r == '_'
	})
}
