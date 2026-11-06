package translate

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Formality is how a translation addresses the reader. Empty leaves it to the
// models, which don't keep to one: they mixed tú and usted in Spanish texts.
type Formality string

// The forms of address a locale can ask for
const (
	Formal   Formality = "formal"
	Informal Formality = "informal"
)

// form is how one form of address shows in a language.
type form struct {
	// pronoun names the form in the prompt. The models keep to it better than to
	// "politely": that put Italian into the plural voi and mistranslated Dutch.
	pronoun string
	// words mark the form wherever they stand, in any case.
	words []string
	// capitalized mark the form in this case, except where a sentence starts and
	// they may mean something else: German "Sie" is also "they" there.
	capitalized []string
}

// forms are the languages whose forms of address their words tell apart, by the
// names the models know them by. Words both forms share, such as Spanish "su", are
// left out, so a text without the others passes.
var forms = map[string]map[Formality]form{
	"German": {
		Formal:   {pronoun: "Sie", capitalized: []string{"Sie", "Ihr", "Ihre", "Ihren", "Ihrem", "Ihrer", "Ihres", "Ihnen"}},
		Informal: {pronoun: "du", words: []string{"du", "dich", "dir", "dein", "deine", "deinen", "deinem", "deiner", "deines"}},
	},
	"French": {
		Formal:   {pronoun: "vous", words: []string{"vous", "votre", "vos"}},
		Informal: {pronoun: "tu", words: []string{"tu", "te", "toi", "ton", "ta", "tes"}},
	},
	"Spanish": {
		Formal:   {pronoun: "usted", words: []string{"usted", "ustedes"}},
		Informal: {pronoun: "tú", words: []string{"tú", "tu", "tus", "te", "ti", "contigo"}},
	},
	"Italian": {
		Formal:   {pronoun: "Lei", capitalized: []string{"Lei", "Suo", "Sua", "Suoi", "Sue"}},
		Informal: {pronoun: "tu", words: []string{"tu", "ti", "te", "tuo", "tua", "tuoi", "tue"}},
	},
	"Dutch": {
		Formal:   {pronoun: "u", words: []string{"u", "uw"}},
		Informal: {pronoun: "je", words: []string{"je", "jij", "jou", "jouw"}},
	},
	"Russian": {
		Formal: {pronoun: "вы", words: []string{
			"вы", "вас", "вам", "вами", "ваш", "ваша", "ваше", "ваши",
			"вашего", "вашей", "вашему", "вашим", "ваших", "вашу", "вашем",
		}},
		Informal: {pronoun: "ты", words: []string{
			"ты", "тебя", "тебе", "тобой", "твой", "твоя", "твоё", "твое", "твои",
			"твоего", "твоей", "твоему", "твоим", "твоих", "твою", "твоём", "твоем",
		}},
	},
}

// addressing is how the prompt's style asks for the form of address.
func addressing(language string, formality Formality) string {
	if pronoun := forms[language][formality].pronoun; pronoun != "" {
		return "addressing the user as " + pronoun
	}

	if formality == Formal {
		return "addressing the user politely"
	}

	return "addressing the user casually"
}

// wordPattern matches the words of a text; placeholders are removed before.
var wordPattern = regexp.MustCompile(`[\p{L}\p{M}]+`)

// WrongFormality returns the first word of text that addresses the reader in the
// other form than formality, or "" for none. Languages without known forms pass.
func WrongFormality(language string, formality Formality, text string) string {
	var other Formality

	switch formality {
	case Formal:
		other = Informal
	case Informal:
		other = Formal
	default:
		return ""
	}

	wrong, ok := forms[language][other]
	if !ok {
		return ""
	}

	// {name} is no word of the text, even when it reads like one.
	text = placeholderPattern.ReplaceAllString(text, "_")

	for _, bounds := range wordPattern.FindAllStringIndex(text, -1) {
		word := text[bounds[0]:bounds[1]]

		if slices.Contains(wrong.words, strings.ToLower(word)) {
			return word
		}

		if slices.Contains(wrong.capitalized, word) && !startsSentence(text[:bounds[0]]) {
			return word
		}
	}

	return ""
}

// startsSentence tells whether a word after before starts a sentence: it follows
// nothing, a sentence end, a colon or a line break, perhaps with opening quotes.
func startsSentence(before string) bool {
	before = strings.TrimRightFunc(before, func(r rune) bool {
		return r == ' ' || r == '\t' || unicode.Is(unicode.Pi, r) || unicode.Is(unicode.Ps, r) || r == '"' || r == '\''
	})
	if before == "" {
		return true
	}

	last, _ := utf8.DecodeLastRuneInString(before)

	return strings.ContainsRune(".!?…:\n\r", last)
}
