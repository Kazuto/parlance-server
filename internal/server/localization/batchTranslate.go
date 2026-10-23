package localization

import (
	"context"
	"log"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/translate"
	"github.com/kazuto/parlance-server/internal/webhook"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// maxBatchEntries keeps a batch within the server's request timeout.
const maxBatchEntries = 50

// BatchTranslate machine-translates entries that have no translation in the
// target locale yet, and saves the results. Existing translations are kept, and
// a result a reviewer would have to check, e.g. one that lost a placeholder,
// counts as a failure instead of being saved.
func (s *Server) BatchTranslate(
	ctx context.Context,
	req *connect.Request[pb.BatchTranslateRequest],
) (*connect.Response[pb.BatchTranslateResponse], error) {
	// Translating loads the translation servers, so only some roles may ask for it.
	if len(req.Msg.EntryIds) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry ids are required")
	}

	if len(req.Msg.EntryIds) > maxBatchEntries {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many entries")
	}

	if req.Msg.TargetLocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "target locale id is required")
	}

	if req.Msg.SourceLocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "source locale id is required")
	}

	translator, err := s.translator(ctx)
	if err != nil {
		return nil, err
	}

	targetLocale, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.TargetLocaleId, errors.ErrLocaleNotFound, "target locale not found")
	if err != nil {
		return nil, err
	}

	// Extract user ID from context
	userID := middleware.UserIDPtr(ctx)

	var successCount int32

	var failureCount int32

	var skippedCount int32

	var resultLocalizations []models.Localization

	for _, entryID := range req.Msg.EntryIds {
		var existing int64

		s.db.Model(&models.Localization{}).
			Where("entry_id = ? AND locale_id = ?", entryID, req.Msg.TargetLocaleId).
			Count(&existing)

		if existing > 0 {
			skippedCount++

			continue
		}

		var entry models.Entry
		if err := s.db.First(&entry, "id = ?", entryID).Error; err != nil {
			failureCount++

			continue
		}

		var sourceLocalization models.Localization
		if err := s.db.Where("entry_id = ? AND locale_id = ?", entryID, req.Msg.SourceLocaleId).First(&sourceLocalization).Error; err != nil {
			failureCount++

			continue
		}

		terms := map[string][]translate.Term{}

		if req.Msg.ApplyTerminology {
			var err error

			terms, err = s.glossaryTerms(sourceLocalization.Translation, []string{targetLocale.ID})
			if err != nil {
				failureCount++

				continue
			}
		}

		suggestion := SuggestOne(ctx, translator, sourceLocalization.Translation, targetLocale, terms[targetLocale.ID])
		if suggestion.ErrorCode != "" || len(suggestion.Warnings) > 0 {
			failureCount++

			continue
		}

		// Machine translations always wait for a reviewer
		targetLocalization := models.Localization{
			EntryID:     entryID,
			LocaleID:    req.Msg.TargetLocaleId,
			Translation: suggestion.Translation,
			Review:      models.Review{Status: models.StatusDraft},
		}
		targetLocalization.CreatedBy = userID

		if err := s.db.Create(&targetLocalization).Error; err != nil {
			failureCount++

			continue
		}

		successCount++

		resultLocalizations = append(resultLocalizations, targetLocalization)
	}

	// One event for the whole batch, as its drafts were saved one by one.
	drafted := make([]string, len(resultLocalizations))
	for i := range resultLocalizations {
		drafted[i] = resultLocalizations[i].ID
	}

	if err := webhook.EmitTranslations(s.db.WithContext(ctx), models.WebhookEventTranslationsAwaitingApproval, drafted, userID, nil); err != nil {
		log.Printf("batch translate: send the drafts to webhooks: %v", err)
	}

	return connect.NewResponse(&pb.BatchTranslateResponse{
		Localizations: converter.LocalizationsToProto(resultLocalizations),
		SuccessCount:  successCount,
		FailureCount:  failureCount,
		SkippedCount:  skippedCount,
	}), nil
}
