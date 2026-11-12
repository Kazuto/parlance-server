package langnames

import "testing"

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
