package translate

import (
	"fmt"
	"regexp"
	"strings"
)

// The templates follow the Hy-MT2 model card, which the models were trained on.
// See https://github.com/Tencent-Hunyuan/Hy-MT2.

// uiStyle is the style every translation is asked for. Without it the models
// translate short texts as nouns, e.g. "Save changes" as "Speichern der Änderungen"
// instead of "Änderungen speichern".
const uiStyle = "software user interface text"

// Prompt builds the prompt for a request: the style template, after the glossary
// terms that occur in the text. Placeholders are masked by the caller, see Mask.
//
// A form of address goes into the style, e.g. "formal, addressing the user as Sie".
//
// The style template ends with the default template's "only output" sentence.
// Without it the models take a short text that reads like an instruction as one:
// they answered "Translate" with "Please translate" and "Language" in Portuguese
// with a translation of the prompt.
func Prompt(req Request) string {
	prompt := fmt.Sprintf(
		"Please translate the following text into %s. Note that the translation style must strictly conform to [%s]. Only output the translated result without any additional explanation:\n\n%s",
		req.TargetLang, style(req), req.Text,
	)
	if len(req.Terms) == 0 {
		return prompt
	}

	var refs strings.Builder

	refs.WriteString("Reference the following translations:\n")

	for _, term := range req.Terms {
		fmt.Fprintf(&refs, "%s translates to %s\n", term.Source, term.Target)
	}

	return refs.String() + "\n" + prompt
}

// style is the style the translation is asked for, with its form of address.
func style(req Request) string {
	if req.Formality != Formal && req.Formality != Informal {
		return uiStyle
	}

	return uiStyle + ", " + string(req.Formality) + ", " + addressing(req.TargetLang, req.Formality)
}

// placeholderPattern matches interpolations such as {name} or {count}.
var placeholderPattern = regexp.MustCompile(`\{[^{}]+\}`)
