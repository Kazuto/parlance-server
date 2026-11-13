package langnames

import (
	"testing"
)

func TestRegion(t *testing.T) {
	tests := []struct {
		code     string
		id, name string
		ok       bool
	}{
		{"pt-BR", "BR", "Brazil", true},
		{"en_gb", "GB", "United Kingdom", true},
		{"es-419", "419", "Latin America", true},
		{"sr-Latn-RS", "RS", "Serbia", true},
		{"de", "", "", false},
		{"zh-Hant", "", "", false},
		{"", "", "", false},
		{"not a code", "", "", false},
	}
	for _, tt := range tests {
		id, name, ok := Region(tt.code)
		if id != tt.id || name != tt.name || ok != tt.ok {
			t.Errorf("Region(%q) = %q, %q, %v; want %q, %q, %v", tt.code, id, name, ok, tt.id, tt.name, tt.ok)
		}
	}
}

func TestValid(t *testing.T) {
	for code, want := range map[string]bool{
		"sv":           true,
		"pt-BR":        true,
		"pt_BR":        true,
		"de-de":        true,
		"es-419":       true,
		"zh-Hant":      true,
		"fil":          true,
		"en-informal":  false,
		"xx":           false,
		"und":          false,
		"x-klingon":    false,
		"readme":       false,
		"._de":         false,
		"translations": false,
		"sr-Latn-RS-x": false,
		"":             false,
	} {
		if got := Valid(code); got != want {
			t.Errorf("Valid(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestName(t *testing.T) {
	tests := []struct{ code, in, want string }{
		{"sv", "en", "Swedish"},
		{"sv", "de", "Schwedisch"},
		{"sv", "fr", "Suédois"},
		{"sv", "ja", "スウェーデン語"},
		{"pt-BR", "en", "Brazilian Portuguese"},
		{"pt_BR", "de-DE", "Brasilianisches Portugiesisch"},
		{"de-de", "en", "German (Germany)"},
		{"abc", "en", ""},
		{"xx", "en", ""},
		{"sv", "xx", ""},
	}
	for _, tt := range tests {
		if got := Name(tt.code, tt.in); got != tt.want {
			t.Errorf("Name(%q, %q) = %q, want %q", tt.code, tt.in, got, tt.want)
		}
	}
}
