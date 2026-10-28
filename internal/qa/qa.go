// Package qa checks a translation against its source text for what a reviewer
// would catch: placeholders, plural forms, markup, the edges of the text and the
// form of address. It mirrors the web app's lib/qa.ts, so the entries filter finds
// what the editor flags.
package qa

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kazuto/parlance-server/internal/translate"
)

// Kinds of Warning, as the web app names them
const (
	PlaceholderMissing    = "PLACEHOLDER_MISSING"
	PlaceholderUnexpected = "PLACEHOLDER_UNEXPECTED"
	PluralForms           = "PLURAL_FORMS"
	LineBreaks            = "LINE_BREAKS"
	WhitespaceStart       = "WHITESPACE_START"
	WhitespaceEnd         = "WHITESPACE_END"
	TagMissing            = "TAG_MISSING"
	TagUnexpected         = "TAG_UNEXPECTED"
	PunctuationEnd        = "PUNCTUATION_END"
	Formality             = "FORMALITY"
)

// Warning is something to check, with what is affected, e.g. the placeholder "{count}".
type Warning struct {
	Code   string
	Detail string
}

// Target is the language a translation is in, for the checks that depend on it.
type Target struct {
	Code      string
	Formality translate.Formality
}

// placeholderPattern matches {name}, printf's %s and %1$d, and Laravel's :name.
// The web app skips a colon right after a letter, digit or colon, as in "10:30" or
// "https://", with a lookbehind that Go lacks; placeholders filters those instead.
var placeholderPattern = regexp.MustCompile(`\{[^{}]+\}|%(?:\d+\$)?[-+0#]*\d*(?:\.\d+)?[sdifuxXoeEgGc]|:[A-Za-z_]\w*`)

var tagPattern = regexp.MustCompile(`<(/?)([A-Za-z][\w-]*)(?:\s[^<>]*)?/?>`)

// foldedMarks are marks that end a sentence in other scripts, by their ASCII forms.
var foldedMarks = map[rune]string{
	'。': ".", '．': ".", '｡': ".", '।': ".", '۔': ".",
	'！': "!", '？': "?", '؟': "?", '：': ":",
}

var terminalMarks = []string{".", "!", "?", ":", "…"}

func placeholders(text string) []string {
	// "%%" is a literal percent sign, not the start of one.
	text = strings.ReplaceAll(text, "%%", "")

	var found []string

	for _, bounds := range placeholderPattern.FindAllStringIndex(text, -1) {
		match := text[bounds[0]:bounds[1]]

		if strings.HasPrefix(match, ":") && bounds[0] > 0 {
			before, _ := utf8.DecodeLastRuneInString(text[:bounds[0]])
			if unicode.IsLetter(before) || unicode.IsNumber(before) || before == ':' {
				continue
			}
		}

		found = append(found, match)
	}

	return found
}

func tags(text string) []string {
	var found []string

	for _, match := range tagPattern.FindAllStringSubmatch(text, -1) {
		found = append(found, "<"+match[1]+strings.ToLower(match[2])+">")
	}

	return found
}

// without returns what is in values but not in other, counting repeats.
func without(values, other []string) []string {
	rest := slices.Clone(other)

	var out []string

	for _, value := range values {
		if index := slices.Index(rest, value); index >= 0 {
			rest = slices.Delete(rest, index, index+1)

			continue
		}

		out = append(out, value)
	}

	return out
}

func unique(values []string) []string {
	var out []string

	for _, value := range values {
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}

	return out
}

// ending returns the mark the text ends with: a sentence end, "other" punctuation,
// or "" for none.
func ending(text string) string {
	last, size := utf8.DecodeLastRuneInString(strings.TrimRightFunc(text, unicode.IsSpace))
	if size == 0 {
		return ""
	}

	mark, ok := foldedMarks[last]
	if !ok {
		mark = string(last)
	}

	if slices.Contains(terminalMarks, mark) {
		return mark
	}

	if unicode.IsPunct(last) {
		return "other"
	}

	return ""
}

func leadingSpace(text string) string {
	return text[:len(text)-len(strings.TrimLeftFunc(text, unicode.IsSpace))]
}

func trailingSpace(text string) string {
	return text[len(strings.TrimRightFunc(text, unicode.IsSpace)):]
}

// Check returns what a reviewer should check in the translation; nothing for blank
// texts. The form of address is checked only for a target that asks for one.
func Check(source, translation string, target Target) []Warning {
	if strings.TrimSpace(source) == "" || strings.TrimSpace(translation) == "" {
		return nil
	}

	var warnings []Warning

	want := placeholders(source)
	got := placeholders(translation)

	for _, placeholder := range unique(want) {
		if !slices.Contains(got, placeholder) {
			warnings = append(warnings, Warning{Code: PlaceholderMissing, Detail: placeholder})
		}
	}

	for _, placeholder := range unique(got) {
		if !slices.Contains(want, placeholder) {
			warnings = append(warnings, Warning{Code: PlaceholderUnexpected, Detail: placeholder})
		}
	}

	if strings.Count(source, "|") != strings.Count(translation, "|") {
		warnings = append(warnings, Warning{Code: PluralForms})
	}

	// Breaks at the edges are whitespace, reported below.
	if !strings.Contains(strings.TrimSpace(source), "\n") && strings.Contains(strings.TrimSpace(translation), "\n") {
		warnings = append(warnings, Warning{Code: LineBreaks})
	}

	if leadingSpace(source) != leadingSpace(translation) {
		warnings = append(warnings, Warning{Code: WhitespaceStart})
	}

	if trailingSpace(source) != trailingSpace(translation) {
		warnings = append(warnings, Warning{Code: WhitespaceEnd})
	}

	sourceTags := tags(source)
	translationTags := tags(translation)

	for _, tag := range unique(without(sourceTags, translationTags)) {
		warnings = append(warnings, Warning{Code: TagMissing, Detail: tag})
	}

	for _, tag := range unique(without(translationTags, sourceTags)) {
		warnings = append(warnings, Warning{Code: TagUnexpected, Detail: tag})
	}

	// Only a sentence end against no mark at all: other marks are often how the
	// language writes it, such as the Greek question mark ";" or a closing quote.
	sourceEnd := ending(source)
	translationEnd := ending(translation)

	if (slices.Contains(terminalMarks, sourceEnd) && translationEnd == "") ||
		(sourceEnd == "" && slices.Contains(terminalMarks, translationEnd)) {
		warnings = append(warnings, Warning{Code: PunctuationEnd, Detail: sourceEnd})
	}

	if language, ok := translate.LanguageName(target.Code); ok {
		if word := translate.WrongFormality(language, target.Formality, translation); word != "" {
			warnings = append(warnings, Warning{Code: Formality, Detail: word})
		}
	}

	return warnings
}
