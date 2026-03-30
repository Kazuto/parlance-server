package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// TranslateLocalization uses AI to translate from source locale to target locale
func (s *Server) TranslateLocalization(
	ctx context.Context,
	req *connect.Request[pb.TranslateLocalizationRequest],
) (*connect.Response[pb.TranslateLocalizationResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	if req.Msg.TargetLocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target locale id is required"))
	}

	if req.Msg.SourceLocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("source locale id is required"))
	}

	// Get source localization
	var sourceLocalization models.Localization
	if err := s.db.Where("entry_id = ? AND locale_id = ?", req.Msg.EntryId, req.Msg.SourceLocaleId).First(&sourceLocalization).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("source localization not found"))
	}

	// TODO: Implement DeepL API integration
	// For now, return a placeholder translation
	translatedText := fmt.Sprintf("[AI-Translated from source] %s", sourceLocalization.Translation)

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)

	// Check if target localization already exists
	var targetLocalization models.Localization
	err := s.db.Where("entry_id = ? AND locale_id = ?", req.Msg.EntryId, req.Msg.TargetLocaleId).First(&targetLocalization).Error

	if err != nil {
		// Create new localization
		targetLocalization = models.Localization{
			EntryID:     req.Msg.EntryId,
			LocaleID:    req.Msg.TargetLocaleId,
			Translation: translatedText,
		}

		if userID != "" {
			targetLocalization.CreatedBy = &userID
		}

		if err := s.db.Create(&targetLocalization).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create translation: %w", err))
		}
	} else {
		// Update existing localization
		if userID != "" {
			targetLocalization.UpdatedBy = &userID
		}

		targetLocalization.Translation = translatedText

		if err := s.db.Save(&targetLocalization).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update translation: %w", err))
		}
	}

	return connect.NewResponse(&pb.TranslateLocalizationResponse{
		Localization: converter.LocalizationToProto(&targetLocalization),
	}), nil
}
