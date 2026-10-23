package localization

import (
	"context"
	stderrors "errors"
	"log"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
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

// suggest translates source into each locale, at most a few at once. The result
// has one suggestion per locale, in the same order; locales not done within
// suggestTimeout report a timeout.
func (s *Server) suggest(
	ctx context.Context,
	translator translate.Translator,
	source string,
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
	source string,
	locale models.Locale,
	terms []translate.Term,
) *pb.TranslationSuggestion {
	suggestion := &pb.TranslationSuggestion{LocaleId: locale.ID}

	language, ok := translate.LanguageName(locale.Code)
	if !ok {
		suggestion.ErrorCode = suggestionUnsupportedLanguage

		return suggestion
	}

	answer, err := translator.Translate(ctx, translate.Request{
		Text:       source,
		TargetLang: language,
		Terms:      terms,
	})
	if err != nil {
		log.Printf("translate into %s: %v", locale.Code, err)

		suggestion.ErrorCode = suggestionFailed
		if isTimeout(ctx, err) {
			suggestion.ErrorCode = suggestionTimeout
		}

		return suggestion
	}

	text, warnings, err := translate.Clean(source, answer)

	switch {
	case stderrors.Is(err, translate.ErrEmptyAnswer):
		suggestion.ErrorCode = suggestionEmptyAnswer
	case stderrors.Is(err, translate.ErrAnswerTooLong):
		suggestion.ErrorCode = suggestionAnswerTooLong
	case err != nil:
		suggestion.ErrorCode = suggestionFailed
	default:
		suggestion.Translation = text
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
func (s *Server) glossaryTerms(source string, localeIDs []string) (map[string][]translate.Term, error) {
	glossary, err := LoadGlossary(s.db.DB, localeIDs)
	if err != nil {
		return nil, err
	}

	return glossary.Terms(source), nil
}

// Glossary is the glossary with its approved translations in some locales.
type Glossary []models.Terminology

// LoadGlossary loads the terms with their approved translations in the locales.
func LoadGlossary(db *gorm.DB, localeIDs []string) (Glossary, error) {
	var terminologies []models.Terminology
	if err := db.Preload("Definitions", "locale_id IN ? AND status = ?", localeIDs, models.StatusApproved).Find(&terminologies).Error; err != nil {
		return nil, err
	}

	return terminologies, nil
}

// Terms returns, per locale, the terms that occur in source with their translation.
func (g Glossary) Terms(source string) map[string][]translate.Term {
	words := translate.Words(source)
	terms := map[string][]translate.Term{}

	for _, terminology := range g {
		if len(translate.FindTerm(terminology.Term, words)) == 0 {
			continue
		}

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

	return terms
}
