package translate

import "testing"

func TestLanguageName(t *testing.T) {
	tests := []struct {
		code string
		want string
		ok   bool
	}{
		{"de", "German", true},
		{"DE", "German", true},
		{"pt-BR", "Portuguese", true},
		{"en_US", "English", true},
		{"zh", "Chinese", true},
		{"zh-CN", "Chinese", true},
		{"zh-Hans", "Chinese", true},
		{"zh-TW", "Traditional Chinese", true},
		{"zh-Hant-HK", "Traditional Chinese", true},
		{"yue", "Cantonese", true},
		{"fil", "Filipino", true},
		{"sv", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := LanguageName(tt.code)
		if got != tt.want || ok != tt.ok {
			t.Errorf("LanguageName(%q) = %q, %v; want %q, %v", tt.code, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTargetName(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		// Variants by the names they go by
		{"en-US", "American English"},
		{"en_gb", "British English"},
		{"pt-BR", "Brazilian Portuguese"},
		{"pt-Latn-BR", "Brazilian Portuguese"},
		{"es-419", "Latin American Spanish"},
		{"de-AT", "Austrian German"},
		{"de-CH", "Swiss Standard German"},
		{"nl-BE", "Flemish"},
		// Other regions in brackets
		{"it-CH", "Italian (Switzerland)"},
		{"en-IN", "English (India)"},
		{"ar-EG", "Arabic (Egypt)"},
		// Codes without a region
		{"de", "German"},
		{"pt", "Portuguese"},
		// Chinese and Cantonese name their scripts, not regions
		{"zh-CN", "Chinese"},
		{"zh-TW", "Traditional Chinese"},
		{"yue-HK", "Cantonese"},
		// Languages the models don't support
		{"sv-SE", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := TargetName(tt.code); got != tt.want {
			t.Errorf("TargetName(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}
