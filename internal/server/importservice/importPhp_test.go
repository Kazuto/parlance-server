package importservice

import (
	"reflect"
	"testing"
)

func TestParsePHPArray(t *testing.T) {
	// The first pairs are in the format the PHP export writes.
	content := `<?php

return [
    'greeting' => 'It\'s in C:\\temp\\',
    'quote' => 'Say "hi"',
    'menu.open' => 'Open',
    "double" => "Line\nTab\t\"quoted\" \$5",
    'kept' => 'a\nb',
];
`
	want := map[string]string{
		"greeting":  `It's in C:\temp\`,
		"quote":     `Say "hi"`,
		"menu.open": "Open",
		"double":    "Line\nTab\t\"quoted\" $5",
		"kept":      `a\nb`,
	}

	got, err := parsePHPArray(content)
	if err != nil {
		t.Fatalf("parsePHPArray() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePHPArray() = %#v, want %#v", got, want)
	}

	if _, err := parsePHPArray("<?php return [];"); err == nil {
		t.Error("expected an error for an array without pairs")
	}
}
