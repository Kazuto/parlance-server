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
func Prompt(req Request) string {
	prompt := fmt.Sprintf(
		"Please translate the following text into %s. Note that the translation style must strictly conform to [%s]:\n\n%s",
		req.TargetLang, uiStyle, req.Text,
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

// placeholderPattern matches interpolations such as {name} or {count}.
var placeholderPattern = regexp.MustCompile(`\{[^{}]+\}`)
