package translate

import (
	"reflect"
	"testing"
)

func TestWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "simple sentence",
			text:     "Delete logo and return",
			expected: []string{"delete", "logo", "and", "return"},
		},
		{
			name:     "with punctuation",
			text:     "Delete logo, return to dashboard!",
			expected: []string{"delete", "logo", "return", "to", "dashboard"},
		},
		{
			name:     "with special characters",
			text:     "Upload a new (logo) - company branding",
			expected: []string{"upload", "a", "new", "logo", "company", "branding"},
		},
		{
			name:     "mixed case",
			text:     "Delete LOGO and Return To Dashboard",
			expected: []string{"delete", "logo", "and", "return", "to", "dashboard"},
		},
		{
			name:     "with quotes",
			text:     `Click "Submit" button`,
			expected: []string{"click", "submit", "button"},
		},
		// Placeholders stay untranslated, so their names are no words of the text.
		{
			name:     "without placeholders",
			text:     "Choose a language for {language} or :language",
			expected: []string{"choose", "a", "language", "for", "or"},
		},
		{
			name:     "colons that are text",
			text:     "Note: open https://parlance.dev at 10:30, key:value",
			expected: []string{"note", "open", "https", "//parlance", "dev", "at", "10", "30", "key", "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Words(tt.text)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Words() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindTerm(t *testing.T) {
	tests := []struct {
		name        string
		term        string
		sourceWords []string
		expected    []string
	}{
		{
			name:        "exact single word match",
			term:        "logo",
			sourceWords: []string{"delete", "logo", "and", "return"},
			expected:    []string{"logo"},
		},
		{
			name:        "no match",
			term:        "api",
			sourceWords: []string{"delete", "logo", "and", "return"},
			expected:    nil,
		},
		{
			name:        "multiple word term match",
			term:        "user profile",
			sourceWords: []string{"edit", "user", "profile", "settings"},
			expected:    []string{"user profile"},
		},
		{
			name:        "case insensitive match",
			term:        "Dashboard",
			sourceWords: []string{"return", "to", "dashboard", "home"},
			expected:    []string{"dashboard"},
		},
		{
			name:        "multiple occurrences",
			term:        "logo",
			sourceWords: []string{"upload", "logo", "or", "remove", "logo"},
			expected:    []string{"logo", "logo"},
		},
		{
			name:        "multi-word no match",
			term:        "user settings",
			sourceWords: []string{"edit", "user", "profile", "and", "settings"},
			expected:    nil, // "user" and "settings" are not sequential
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindTerm(tt.term, tt.sourceWords)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("FindTerm() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindTermWithRealExamples(t *testing.T) {
	// Real-world example from the use case
	sourceText := "Delete logo and return to dashboard"
	sourceWords := Words(sourceText)

	// Test "Logo" terminology
	logoMatches := FindTerm("Logo", sourceWords)
	if len(logoMatches) != 1 || logoMatches[0] != "logo" {
		t.Errorf("Expected to find 'logo', got %v", logoMatches)
	}

	// Test "Dashboard" terminology
	dashboardMatches := FindTerm("Dashboard", sourceWords)
	if len(dashboardMatches) != 1 || dashboardMatches[0] != "dashboard" {
		t.Errorf("Expected to find 'dashboard', got %v", dashboardMatches)
	}

	// Test non-existent terminology
	apiMatches := FindTerm("API", sourceWords)
	if len(apiMatches) != 0 {
		t.Errorf("Expected no matches for 'API', got %v", apiMatches)
	}
}
