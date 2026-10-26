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
	// Formality is how the translation addresses the reader; empty for either.
	Formality Formality
}

// Answer is the model's raw answer, to be checked with Clean.
type Answer struct {
	Text string
	// Model names the model that wrote it, as its server reports it, e.g.
	// "tencent/Hy-MT2-7B-GGUF:Q4_K_M"; empty when unknown.
	Model string
}

// Translator asks a model for a translation.
type Translator interface {
	Translate(ctx context.Context, req Request) (Answer, error)
}

// ModelLister is a translator that can tell which models it uses, so translations
// by other models can be found, e.g. to redo them after switching models.
type ModelLister interface {
	Models(ctx context.Context) ([]string, error)
}
