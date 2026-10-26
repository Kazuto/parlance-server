// Package termmatch finds glossary terms in texts by their word stems, so a term
// matches its other forms: "scope" in "No scopes", German "Bereich" in "Keine
// Bereiche". It uses Postgres's text search stemmers, which cover about thirty
// languages; others match whole words exactly, and languages written without spaces,
// such as Chinese and Japanese, match the term anywhere in the text.
package termmatch

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kazuto/parlance-server/internal/translate"
	"gorm.io/gorm"
)

// configs are Postgres's text search configurations by language code. Each stems
// its language's words and leaves out its stop words.
var configs = map[string]string{
	"ar": "arabic", "hy": "armenian", "eu": "basque", "ca": "catalan", "da": "danish",
	"nl": "dutch", "en": "english", "fi": "finnish", "fr": "french", "de": "german",
	"el": "greek", "hi": "hindi", "hu": "hungarian", "id": "indonesian", "ga": "irish",
	"it": "italian", "lt": "lithuanian", "ne": "nepali", "no": "norwegian", "nb": "norwegian",
	"nn": "norwegian", "pt": "portuguese", "ro": "romanian", "ru": "russian", "sr": "serbian",
	"es": "spanish", "sv": "swedish", "ta": "tamil", "tr": "turkish", "yi": "yiddish",
}

// unspaced are languages written without spaces between words, where a term is
// found anywhere in the text.
var unspaced = map[string]bool{"zh": true, "ja": true, "th": true, "lo": true, "km": true, "my": true}

// language returns the base language of a code such as "pt-BR".
func language(code string) string {
	base, _, _ := strings.Cut(strings.ToLower(code), "-")
	base, _, _ = strings.Cut(base, "_")

	return base
}

// Config returns the text search configuration for a locale code: its stemmer, or
// "simple", which matches whole words.
func Config(code string) string {
	if config, ok := configs[language(code)]; ok {
		return config
	}

	return "simple"
}

// Term is a text to find, by an id of the caller's.
type Term struct {
	ID   string
	Text string
}

// Matcher finds terms of one language in texts of that language.
type Matcher struct {
	db       *gorm.DB
	config   string
	unspaced bool
	// stemmed reads text through Postgres; without it, words match exactly, as
	// SQLite in tests has no text search.
	stemmed bool
	terms   []Term
	keys    map[string][]string
}

// word is a word of a text, with the key it matches by: its stem, or for a stop
// word, itself.
type word struct {
	text string
	key  string
}

// New prepares the terms of the language of code for finding them in texts.
func New(ctx context.Context, db *gorm.DB, code string, terms []Term) (*Matcher, error) {
	m := &Matcher{
		db:       db.WithContext(ctx),
		config:   Config(code),
		unspaced: unspaced[language(code)],
		stemmed:  db.Name() == "postgres",
		terms:    terms,
		keys:     make(map[string][]string, len(terms)),
	}

	if m.unspaced || len(terms) == 0 {
		return m, nil
	}

	texts := make([]string, len(terms))
	for i, term := range terms {
		texts[i] = term.Text
	}

	stemmed, err := m.words(texts)
	if err != nil {
		return nil, err
	}

	for i, term := range terms {
		keys := make([]string, len(stemmed[i]))
		for j, w := range stemmed[i] {
			keys[j] = w.key
		}

		m.keys[term.ID] = keys
	}

	return m, nil
}

// Find returns the terms that occur in text, by id, each with the words of text it
// matched, in lower case.
func (m *Matcher) Find(text string) (map[string][]string, error) {
	found := map[string][]string{}

	if m.unspaced {
		normalized := strings.ToLower(withoutPlaceholders(text))

		for _, term := range m.terms {
			needle := strings.ToLower(strings.TrimSpace(term.Text))
			if needle != "" && strings.Contains(normalized, needle) {
				found[term.ID] = []string{needle}
			}
		}

		return found, nil
	}

	stemmed, err := m.words([]string{text})
	if err != nil {
		return nil, err
	}

	words := stemmed[0]

	for _, term := range m.terms {
		keys := m.keys[term.ID]
		if len(keys) == 0 {
			continue
		}

		for start := 0; start+len(keys) <= len(words); start++ {
			if !matchesAt(words, start, keys) {
				continue
			}

			parts := make([]string, len(keys))
			for i := range keys {
				parts[i] = strings.ToLower(words[start+i].text)
			}

			found[term.ID] = append(found[term.ID], strings.Join(parts, " "))
		}
	}

	return found, nil
}

func matchesAt(words []word, start int, keys []string) bool {
	for i, key := range keys {
		if words[start+i].key != key {
			return false
		}
	}

	return true
}

// withoutPlaceholders blanks out placeholders, which stay as they are, so "{name}"
// doesn't mention the term "Name".
func withoutPlaceholders(text string) string {
	return strings.Join(translate.Words(text), " ")
}

// words splits each text into its words with their stems, in one query.
func (m *Matcher) words(texts []string) ([][]word, error) {
	result := make([][]word, len(texts))
	plain := make([]string, len(texts))

	for i, text := range texts {
		plain[i] = withoutPlaceholders(text)
	}

	if !m.stemmed {
		for i, text := range plain {
			for w := range strings.FieldsSeq(text) {
				result[i] = append(result[i], word{text: w, key: w})
			}
		}

		return result, nil
	}

	encoded, err := json.Marshal(plain)
	if err != nil {
		return nil, err
	}

	var tokens []struct {
		Ord   int
		Token string
		Stem  *string
	}

	// A hyphenated word comes as a whole and as its parts; only the parts count, so
	// "E-Mail" matches "E-Mail-Adresse". Stop words have no stem and match themselves.
	err = m.db.Raw(`SELECT t.ord, d.token, d.lexemes[1] AS stem
		FROM jsonb_array_elements_text(?::jsonb) WITH ORDINALITY AS t(text, ord),
			LATERAL ts_debug(?::regconfig, t.text) WITH ORDINALITY AS d(alias, description, token, dictionaries, dictionary, lexemes, pos)
		WHERE d.lexemes IS NOT NULL AND d.alias NOT IN ('asciihword', 'hword', 'numhword')
		ORDER BY t.ord, d.pos`, string(encoded), m.config).
		Scan(&tokens).Error
	if err != nil {
		return nil, err
	}

	for _, token := range tokens {
		key := strings.ToLower(token.Token)
		if token.Stem != nil && *token.Stem != "" {
			key = *token.Stem
		}

		result[token.Ord-1] = append(result[token.Ord-1], word{text: token.Token, key: key})
	}

	return result, nil
}
