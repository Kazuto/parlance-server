// Package translate suggests machine translations for single strings.
//
// It is built around Tencent's Hy-MT2 translation models, which translate one
// text into one language per request and answer with the bare translation. The
// client speaks the OpenAI chat API, so any server for those models works:
// llama.cpp, vLLM, SGLang or a hosted provider.
package translate

import "context"

// Term is a glossary entry the translation should follow.
type Term struct {
	Source string
	Target string
}

// Request asks for one text in one language.
type Request struct {
	Text string
	// TargetLang is the English language name the model expects, see LanguageName.
	TargetLang string
	Terms      []Term
}

// Translator returns the model's raw answer, to be checked with Clean.
type Translator interface {
	Translate(ctx context.Context, req Request) (string, error)
}
