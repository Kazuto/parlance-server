package translate

import (
	"math"
	"testing"
)

func TestSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"Delete entries", "Delete entry", 1 - 3.0/14},
		{"Save your changes", "Save changes", 1 - 5.0/17},
		{"Save your changes", "  save YOUR changes ", 1},
		{"Änderungen", "änderungen", 1},
		// Each of these characters is one rune, not several bytes.
		{"日本語", "日本", 1 - 1.0/3},
		{"Save", "", 0},
		{"", "", 1},
		{"Your changes were saved", "Save your changes", 1 - 16.0/23},
	}

	for _, tt := range tests {
		got := Similarity(tt.a, tt.b)
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("Similarity(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}

		if reverse := Similarity(tt.b, tt.a); math.Abs(reverse-got) > 1e-9 {
			t.Errorf("Similarity(%q, %q) = %v, but %v the other way", tt.b, tt.a, reverse, got)
		}
	}
}
