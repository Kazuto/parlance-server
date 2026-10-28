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
