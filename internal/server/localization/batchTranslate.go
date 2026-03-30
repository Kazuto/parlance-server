package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// BatchTranslate translates multiple entries in batch
func (s *Server) BatchTranslate(
	ctx context.Context,
	req *connect.Request[pb.BatchTranslateRequest],
) (*connect.Response[pb.BatchTranslateResponse], error) {
	if len(req.Msg.EntryIds) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry ids are required"))
	}

	if req.Msg.TargetLocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target locale id is required"))
	}

	if req.Msg.SourceLocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("source locale id is required"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)

	var successCount int32
	var failureCount int32
	var resultLocalizations []models.Localization

	for _, entryID := range req.Msg.EntryIds {
		// Get source localization
		var sourceLocalization models.Localization
		if err := s.db.Where("entry_id = ? AND locale_id = ?", entryID, req.Msg.SourceLocaleId).First(&sourceLocalization).Error; err != nil {
			failureCount++
			continue
		}

		// TODO: Implement DeepL API integration
		translatedText := fmt.Sprintf("[AI-Translated from source] %s", sourceLocalization.Translation)

		// Check if target localization already exists
		var targetLocalization models.Localization
		err := s.db.Where("entry_id = ? AND locale_id = ?", entryID, req.Msg.TargetLocaleId).First(&targetLocalization).Error

		if err != nil {
			// Create new localization
			targetLocalization = models.Localization{
				EntryID:     entryID,
				LocaleID:    req.Msg.TargetLocaleId,
				Translation: translatedText,
			}

			if userID != "" {
				targetLocalization.CreatedBy = &userID
			}

			if err := s.db.Create(&targetLocalization).Error; err != nil {
				failureCount++
				continue
			}
		} else {
			// Update existing localization
			if userID != "" {
				targetLocalization.UpdatedBy = &userID
			}

			targetLocalization.Translation = translatedText

			if err := s.db.Save(&targetLocalization).Error; err != nil {
				failureCount++
				continue
			}
		}

		successCount++
		resultLocalizations = append(resultLocalizations, targetLocalization)
	}

	return connect.NewResponse(&pb.BatchTranslateResponse{
		Localizations: converter.LocalizationsToProto(resultLocalizations),
		SuccessCount:  successCount,
		FailureCount:  failureCount,
	}), nil
}
