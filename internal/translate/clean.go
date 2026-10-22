package translate

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"
)

// Reasons an answer can't be used at all.
var (
	ErrEmptyAnswer   = errors.New("the model returned no translation")
	ErrAnswerTooLong = errors.New("the answer is much longer than the source text")
)

// Kinds of Warning. The editor shows them next to the suggestion.
const (
	WarningPlaceholderMissing    = "PLACEHOLDER_MISSING"
	WarningPlaceholderUnexpected = "PLACEHOLDER_UNEXPECTED"
	WarningPluralForms           = "PLURAL_FORMS"
	WarningLineBreaks            = "LINE_BREAKS"
)

// Warning marks a suggestion that is usable but should be checked.
type Warning struct {
	Code string
	// Detail names what is affected, e.g. the placeholder "{count}".
	Detail string
}

// maxLengthRatio limits answers to a few times the source length: longer ones
// are almost always explanations or repetition, not a translation.
const maxLengthRatio = 4

// quotePairs are the quotes a model may wrap its answer in.
var quotePairs = [][2]string{
	{`"`, `"`}, {"'", "'"}, {"“", "”"}, {"„", "“"}, {"«", "»"}, {"「", "」"}, {"`", "`"},
}

// Clean turns the model's answer into a translation of source, and lists what a
// reviewer should check.
func Clean(source, answer string) (string, []Warning, error) {
	text := strings.TrimSpace(answer)
	text = stripFence(source, text)
	text = stripQuotes(source, text)
	if text == "" {
		return "", nil, ErrEmptyAnswer
	}
	if utf8.RuneCountInString(text) > maxLengthRatio*utf8.RuneCountInString(source)+20 {
		return "", nil, ErrAnswerTooLong
	}

	var warnings []Warning
	want := placeholderPattern.FindAllString(source, -1)
	got := placeholderPattern.FindAllString(text, -1)
	for _, placeholder := range uniq(want) {
		if !slices.Contains(got, placeholder) {
			warnings = append(warnings, Warning{Code: WarningPlaceholderMissing, Detail: placeholder})
		}
	}
	for _, placeholder := range uniq(got) {
		if !slices.Contains(want, placeholder) {
			warnings = append(warnings, Warning{Code: WarningPlaceholderUnexpected, Detail: placeholder})
		}
	}
	if strings.Count(source, "|") != strings.Count(text, "|") {
		warnings = append(warnings, Warning{Code: WarningPluralForms})
	}
	if !strings.Contains(source, "\n") && strings.Contains(text, "\n") {
		warnings = append(warnings, Warning{Code: WarningLineBreaks})
	}

	return text, warnings, nil
}

// stripFence removes a code block around the whole answer.
func stripFence(source, text string) string {
	if !strings.HasPrefix(text, "```") || strings.HasPrefix(strings.TrimSpace(source), "```") {
		return text
	}
	text = strings.TrimSuffix(strings.TrimPrefix(text, "```"), "```")
	// Drop a language tag such as ```text on the first line.
	if first, rest, ok := strings.Cut(text, "\n"); ok && !strings.Contains(strings.TrimSpace(first), " ") {
		text = rest
	}
	return strings.TrimSpace(text)
}

// stripQuotes removes quotes around the whole answer, unless the source has them.
func stripQuotes(source, text string) string {
	source = strings.TrimSpace(source)
	for _, pair := range quotePairs {
		opening, closing := pair[0], pair[1]
		if strings.HasPrefix(source, opening) && strings.HasSuffix(source, closing) {
			continue
		}
		if len(text) >= len(opening)+len(closing) && strings.HasPrefix(text, opening) && strings.HasSuffix(text, closing) {
			return strings.TrimSpace(text[len(opening) : len(text)-len(closing)])
		}
	}
	return text
}

// uniq returns the values without repeats, in order.
func uniq(values []string) []string {
	var out []string
	for _, value := range values {
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}
