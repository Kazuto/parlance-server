package localization

import (
	"context"
	stderrors "errors"
	"log"
	"sync"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/translate"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// maxParallelTranslations limits the requests one call sends to the model at once.
const maxParallelTranslations = 4

// Error codes of a TranslationSuggestion.
const (
	suggestionUnsupportedLanguage = "UNSUPPORTED_LANGUAGE"
	suggestionEmptyAnswer         = "EMPTY_ANSWER"
	suggestionAnswerTooLong       = "ANSWER_TOO_LONG"
	suggestionFailed              = "FAILED"
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
// has one suggestion per locale, in the same order.
func (s *Server) suggest(
	ctx context.Context,
	translator translate.Translator,
	source string,
	locales []models.Locale,
	terms map[string][]translate.Term,
) []*pb.TranslationSuggestion {
	suggestions := make([]*pb.TranslationSuggestion, len(locales))
	limit := make(chan struct{}, maxParallelTranslations)
	var wg sync.WaitGroup

	for i, locale := range locales {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			suggestions[i] = suggestOne(ctx, translator, source, locale, terms[locale.ID])
		})
	}

	wg.Wait()
	return suggestions
}

func suggestOne(
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

// glossaryTerms returns, per locale, the glossary terms that occur in source and
// have a translation in that locale.
func (s *Server) glossaryTerms(source string, localeIDs []string) (map[string][]translate.Term, error) {
	var terminologies []models.Terminology
	if err := s.db.Preload("Definitions", "locale_id IN ?", localeIDs).Find(&terminologies).Error; err != nil {
		return nil, err
	}

	words := translate.Words(source)
	terms := map[string][]translate.Term{}
	for _, terminology := range terminologies {
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
	return terms, nil
}
