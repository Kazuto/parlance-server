package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// TranslateLocalization uses AI to translate from source locale to target locale
func (s *Server) TranslateLocalization(
	ctx context.Context,
	req *connect.Request[pb.TranslateLocalizationRequest],
) (*connect.Response[pb.TranslateLocalizationResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	if req.Msg.TargetLocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "target locale id is required")
	}

	if req.Msg.SourceLocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "source locale id is required")
	}

	// Get source localization
	var sourceLocalization models.Localization
	if err := s.db.Where("entry_id = ? AND locale_id = ?", req.Msg.EntryId, req.Msg.SourceLocaleId).First(&sourceLocalization).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocalizationNotFound, "source localization not found")
	}

	// TODO: Implement DeepL API integration
	// For now, return a placeholder translation
	translatedText := fmt.Sprintf("[AI-Translated from source] %s", sourceLocalization.Translation)

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)

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
			return nil, errors.Internal(errors.ErrInternal, "failed to create translation")
		}
	} else {
		// Update existing localization
		if userID != "" {
			targetLocalization.UpdatedBy = &userID
		}

		targetLocalization.Translation = translatedText

		if err := s.db.Save(&targetLocalization).Error; err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to update translation")
		}
	}

	return connect.NewResponse(&pb.TranslateLocalizationResponse{
		Localization: converter.LocalizationToProto(&targetLocalization),
	}), nil
}
