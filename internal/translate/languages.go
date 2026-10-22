package translate

import "strings"

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

// LanguageName returns the model's name for a locale code such as "de" or
// "pt-BR", and false when the models don't support the language.
func LanguageName(code string) (string, bool) {
	parts := strings.FieldsFunc(strings.ToLower(code), func(r rune) bool {
		return r == '-' || r == '_'
	})
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
