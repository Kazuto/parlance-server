package translate

import (
	"regexp"
	"strings"
)

// termPlaceholderPattern matches placeholders, which stay as they are and so don't
// mention a glossary term: {language}, or Laravel's :language. A colon right after
// a letter, digit or colon is text, as in "10:30" or "https://".
var termPlaceholderPattern = regexp.MustCompile(`\{[^{}]+\}|(^|[^\p{L}\p{N}:]):[A-Za-z_]\w*`)

// wordSeparators are replaced by spaces before splitting a text into words.
var wordSeparators = strings.NewReplacer(
	".", " ", ",", " ", "!", " ", "?", " ", ";", " ", ":", " ", "-", " ", "_", " ",
	"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ", "\"", " ", "'", " ",
)

// Words splits text into lowercase words, without punctuation and placeholders.
func Words(text string) []string {
	text = termPlaceholderPattern.ReplaceAllString(text, "${1} ")

	return strings.Fields(wordSeparators.Replace(strings.ToLower(text)))
}

// FindTerm returns where a glossary term occurs in words from Words: as a whole
// word, or for a term of several words, as that sequence of words.
func FindTerm(term string, words []string) []string {
	termLower := strings.ToLower(term)

	var matches []string

	for _, word := range words {
		if word == termLower {
			matches = append(matches, word)
		}
	}

	termWords := strings.Fields(termLower)
	if len(termWords) > 1 {
		for i := 0; i <= len(words)-len(termWords); i++ {
			allMatch := true

			for j, termWord := range termWords {
				if words[i+j] != termWord {
					allMatch = false

					break
				}
			}

			if allMatch {
				matches = append(matches, strings.Join(words[i:i+len(termWords)], " "))
			}
		}
	}

	return matches
}
