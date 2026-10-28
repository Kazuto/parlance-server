package localization

import (
	"context"
	stderrors "errors"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/settings"
	"github.com/kazuto/parlance-server/internal/termmatch"
	"github.com/kazuto/parlance-server/internal/translate"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// maxParallelTranslations limits the requests one call sends to the model at once.
const maxParallelTranslations = 4

// suggestTimeout limits one call, queueing included. It stays below the HTTP
// server's write timeout, so the languages done in time still reach the client.
var suggestTimeout = 100 * time.Second

// Error codes of a TranslationSuggestion.
const (
	suggestionUnsupportedLanguage = "UNSUPPORTED_LANGUAGE"
	suggestionEmptyAnswer         = "EMPTY_ANSWER"
	suggestionAnswerTooLong       = "ANSWER_TOO_LONG"
	suggestionFailed              = "FAILED"
	suggestionTimeout             = "TIMEOUT"
)

// translator returns the translator of the enabled servers, and fails while
// there is none.
func (s *Server) translator(ctx context.Context) (translate.Translator, error) {
	if s.translators == nil {
		return nil, errors.FailedPrecondition(errors.ErrFailedPrecondition, "machine translation is not configured")
	}

	translator, err := s.translators.Translator(ctx)
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to load the translation servers")
	}

	if translator == nil {
		return nil, errors.FailedPrecondition(errors.ErrFailedPrecondition, "machine translation is not configured")
	}

	return translator, nil
}

// Source is a text to machine-translate, with what the model is told about it.
type Source struct {
	Text string
	// Context is the entry's description.
	Context string
	// Style is the translation style admins set; empty for the default.
	Style string
}

// Style returns the translation style admins set, or "" for the default.
func Style(ctx context.Context, db *gorm.DB) (string, error) {
	style, _, err := settings.String(db.WithContext(ctx), models.SettingTranslationStyle, "")

	return style, err
}

// suggest translates source into each locale, at most a few at once. The result
// has one suggestion per locale, in the same order; locales not done within
// suggestTimeout report a timeout.
func (s *Server) suggest(
	ctx context.Context,
	translator translate.Translator,
	source Source,
	locales []models.Locale,
	terms map[string][]translate.Term,
) []*pb.TranslationSuggestion {
	ctx, cancel := context.WithTimeout(ctx, suggestTimeout)
	defer cancel()

	suggestions := make([]*pb.TranslationSuggestion, len(locales))
	limit := make(chan struct{}, maxParallelTranslations)

	var wg sync.WaitGroup

	for i, locale := range locales {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()

			suggestions[i] = SuggestOne(ctx, translator, source, locale, terms[locale.ID])
		})
	}

	wg.Wait()

	return suggestions
}

// SuggestOne machine-translates source into the locale. A suggestion with an error
// code or warnings is one a reviewer shouldn't be handed as a draft.
func SuggestOne(
	ctx context.Context,
	translator translate.Translator,
	source Source,
	locale models.Locale,
	terms []translate.Term,
) *pb.TranslationSuggestion {
	suggestion := &pb.TranslationSuggestion{LocaleId: locale.ID}

	language, ok := translate.LanguageName(locale.Code)
	if !ok {
		suggestion.ErrorCode = suggestionUnsupportedLanguage

		return suggestion
	}

	formality := translate.Formality(locale.Formality)

	answer, err := translator.Translate(ctx, translate.Request{
		Text:       source.Text,
		TargetLang: language,
		Terms:      terms,
		Formality:  formality,
		Context:    source.Context,
		Style:      source.Style,
	})
	if err != nil {
		log.Printf("translate into %s: %v", locale.Code, err)

		suggestion.ErrorCode = suggestionFailed
		if isTimeout(ctx, err) {
			suggestion.ErrorCode = suggestionTimeout
		}

		return suggestion
	}

	text, warnings, err := translate.Clean(source.Text, answer.Text)

	switch {
	case stderrors.Is(err, translate.ErrEmptyAnswer):
		suggestion.ErrorCode = suggestionEmptyAnswer
	case stderrors.Is(err, translate.ErrAnswerTooLong):
		suggestion.ErrorCode = suggestionAnswerTooLong
	case err != nil:
		suggestion.ErrorCode = suggestionFailed
	default:
		if word := translate.WrongFormality(language, formality, text); word != "" {
			warnings = append(warnings, translate.Warning{Code: translate.WarningFormality, Detail: word})
		}

		suggestion.Translation = text
		suggestion.Model = answer.Model

		for _, warning := range warnings {
			suggestion.Warnings = append(suggestion.Warnings, &pb.TranslationWarning{
				Code:   warning.Code,
				Detail: warning.Detail,
			})
		}
	}

	return suggestion
}

// isTimeout tells whether err means the call or the request to the model took too
// long, as opposed to the model failing.
func isTimeout(ctx context.Context, err error) bool {
	if stderrors.Is(ctx.Err(), context.DeadlineExceeded) || stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var timeout interface{ Timeout() bool }

	return stderrors.As(err, &timeout) && timeout.Timeout()
}

// glossaryTerms returns, per locale, the glossary terms that occur in source and
// have an approved translation in that locale, so a draft can't steer translations.
// The source is in the language of sourceCode, whose word forms the terms match.
func (s *Server) glossaryTerms(ctx context.Context, source, sourceCode string, localeIDs []string) (map[string][]translate.Term, error) {
	glossary, err := LoadGlossary(ctx, s.db.DB, sourceCode, localeIDs)
	if err != nil {
		return nil, err
	}

	return glossary.Terms(source)
}

// Glossary is the glossary with its approved translations in some locales, ready
// to find its terms in texts of one language.
type Glossary struct {
	terminologies map[string]models.Terminology
	matcher       *termmatch.Matcher
}

// LoadGlossary loads the terms with their approved translations in the locales, to
// find them in texts in the language of sourceCode, in any of their word forms.
func LoadGlossary(ctx context.Context, db *gorm.DB, sourceCode string, localeIDs []string) (*Glossary, error) {
	var terminologies []models.Terminology
	if err := db.WithContext(ctx).Preload("Definitions", "locale_id IN ? AND status = ?", localeIDs, models.StatusApproved).Find(&terminologies).Error; err != nil {
		return nil, err
	}

	glossary := &Glossary{terminologies: make(map[string]models.Terminology, len(terminologies))}
	terms := make([]termmatch.Term, len(terminologies))

	for i, terminology := range terminologies {
		glossary.terminologies[terminology.ID] = terminology
		terms[i] = termmatch.Term{ID: terminology.ID, Text: terminology.Term}
	}

	matcher, err := termmatch.New(ctx, db, sourceCode, terms)
	if err != nil {
		return nil, err
	}

	glossary.matcher = matcher

	return glossary, nil
}

// Terms returns, per locale, the terms that occur in source with their translation.
func (g *Glossary) Terms(source string) (map[string][]translate.Term, error) {
	terms := map[string][]translate.Term{}

	found, err := g.matcher.Find(source)
	if err != nil {
		return nil, err
	}

	for id := range found {
		terminology := g.terminologies[id]

		for _, definition := range terminology.Definitions {
			if definition.Translation == "" {
				continue
			}

			terms[definition.LocaleID] = append(terms[definition.LocaleID], translate.Term{
				Source: terminology.Term,
				Target: definition.Translation,
			})
		}
	}

	// A fixed order keeps the prompt the same for the same text.
	for localeID := range terms {
		slices.SortFunc(terms[localeID], func(a, b translate.Term) int { return strings.Compare(a.Source, b.Source) })
	}

	return terms, nil
}
