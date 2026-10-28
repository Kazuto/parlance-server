package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/termmatch"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// SuggestTerminologies returns the glossary terms in a source text, in the default
// locale's language, with their translation into the target locale. Terms match in
// any of their word forms, e.g. "scope" in "No scopes". With a translation, each
// suggestion also tells whether the translation uses the term's translation.
func (s *Server) SuggestTerminologies(
	ctx context.Context,
	req *connect.Request[pb.SuggestTerminologiesRequest],
) (*connect.Response[pb.SuggestTerminologiesResponse], error) {
	if req.Msg.SourceText == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "source text is required")
	}

	if req.Msg.TargetLocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "target locale id is required")
	}

	db := s.db.WithContext(ctx)

	targetLocale, err := common.FindByID[models.Locale](db, req.Msg.TargetLocaleId, errors.ErrLocaleNotFound, "target locale not found")
	if err != nil {
		return nil, err
	}

	// Source texts are written in the default locale; without one, words match exactly.
	sourceCode := ""

	if defaultLocale, err := common.DefaultLocale(db); err != nil {
		return nil, err
	} else if defaultLocale != nil {
		sourceCode = defaultLocale.Code
	}

	var terminologies []models.Terminology
	if err := db.Preload("Definitions", "locale_id = ?", req.Msg.TargetLocaleId).Find(&terminologies).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to fetch terminologies")
	}

	terms := make([]termmatch.Term, len(terminologies))
	for i, terminology := range terminologies {
		terms[i] = termmatch.Term{ID: terminology.ID, Text: terminology.Term}
	}

	sourceMatcher, err := termmatch.New(ctx, s.db.DB, sourceCode, terms)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to read the glossary terms")
	}

	found, err := sourceMatcher.Find(req.Msg.SourceText)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to read the source text")
	}

	suggestions := []*pb.TerminologySuggestion{}
	targets := []termmatch.Term{}

	for _, terminology := range terminologies {
		matchedWords, ok := found[terminology.ID]
		if !ok || len(terminology.Definitions) == 0 || terminology.Definitions[0].Translation == "" {
			continue
		}

		// Only terms with a translation for the target locale are suggested.
		definition := terminology.Definitions[0]
		suggestions = append(suggestions, &pb.TerminologySuggestion{
			Terminology:       converter.TerminologyToProto(&terminology),
			TargetTranslation: definition.Translation,
			TargetStatus:      converter.ReviewStatusToProto(definition.Status),
			MatchedWords:      matchedWords,
		})

		targets = append(targets, termmatch.Term{ID: terminology.ID, Text: definition.Translation})
	}

	if req.Msg.Translation != nil && len(suggestions) > 0 {
		if err := markUsed(ctx, s.db.DB, targetLocale.Code, targets, req.Msg.GetTranslation(), suggestions); err != nil {
			return nil, err
		}
	}

	return connect.NewResponse(&pb.SuggestTerminologiesResponse{Suggestions: suggestions}), nil
}

// markUsed sets used on the suggestions whose translation, the targets in the same
// order, occurs in translation in any of its word forms, e.g. "Bereich" in "Keine
// Bereiche".
func markUsed(ctx context.Context, db *gorm.DB, code string, targets []termmatch.Term, translation string, suggestions []*pb.TerminologySuggestion) error {
	matcher, err := termmatch.New(ctx, db, code, targets)
	if err != nil {
		return errors.InternalCause(err, "failed to read the glossary translations")
	}

	used, err := matcher.Find(translation)
	if err != nil {
		return errors.InternalCause(err, "failed to read the translation")
	}

	for i, target := range targets {
		_, suggestions[i].Used = used[target.ID]
	}

	return nil
}
