package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateLocalization updates an existing localization
func (s *Server) UpdateLocalization(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocalizationRequest],
) (*connect.Response[pb.UpdateLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("localization id is required"))
	}

	if req.Msg.Translation == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("translation is required"))
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("localization not found"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		localization.UpdatedBy = &userID
	}

	localization.Translation = req.Msg.Translation

	if err := s.db.Save(&localization).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update localization: %w", err))
	}

	return connect.NewResponse(&pb.UpdateLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
