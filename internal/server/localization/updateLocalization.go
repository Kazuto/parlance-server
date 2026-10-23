package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateLocalization updates an existing localization
func (s *Server) UpdateLocalization(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocalizationRequest],
) (*connect.Response[pb.UpdateLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	localization, err := common.FindByID[models.Localization](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocalizationNotFound, "localization not found")
	if err != nil {
		return nil, err
	}

	// Credit the change, including a deletion, to the current user
	localization.UpdatedBy = middleware.UserIDPtr(ctx)

	// If translation is being set to empty, delete the localization instead
	if req.Msg.Translation != nil && req.Msg.GetTranslation() == "" {
		if err := s.db.Delete(&localization).Error; err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to delete localization")
		}

		return connect.NewResponse(&pb.UpdateLocalizationResponse{
			Localization: nil,
		}), nil
	}

	// New text needs review again
	if req.Msg.Translation != nil && req.Msg.GetTranslation() != localization.Translation {
		localization.Translation = req.Msg.GetTranslation()
		localization.MarkDraft()
	}

	if err := s.db.Save(&localization).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update localization")
	}

	return connect.NewResponse(&pb.UpdateLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
