package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/translate"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

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

	// Verify target locale exists
	if _, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.TargetLocaleId, errors.ErrLocaleNotFound, "target locale not found"); err != nil {
		return nil, err
	}

	// Get all terminologies with their definitions
	var terminologies []models.Terminology
	if err := s.db.Preload("Definitions", "locale_id = ?", req.Msg.TargetLocaleId).Find(&terminologies).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to fetch terminologies")
	}

	// Tokenize source text (simple word-based tokenization)
	sourceWords := translate.Words(req.Msg.SourceText)

	// Find matching terminologies
	var suggestions []*pb.TerminologySuggestion

	for _, term := range terminologies {
		// Check if terminology term appears in source text (case-insensitive)
		matchedWords := translate.FindTerm(term.Term, sourceWords)

		if len(matchedWords) > 0 {
			// Get the definition for the target locale
			var targetTranslation string

			for _, def := range term.Definitions {
				if def.LocaleID == req.Msg.TargetLocaleId {
					targetTranslation = def.Translation

					break
				}
			}

			// Only suggest if we have a translation for the target locale
			if targetTranslation != "" {
				suggestions = append(suggestions, &pb.TerminologySuggestion{
					Terminology:       converter.TerminologyToProto(&term),
					TargetTranslation: targetTranslation,
					MatchedWords:      matchedWords,
				})
			}
		}
	}

	return connect.NewResponse(&pb.SuggestTerminologiesResponse{
		Suggestions: suggestions,
	}), nil
}
