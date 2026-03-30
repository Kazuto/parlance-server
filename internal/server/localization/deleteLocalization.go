package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteLocalization soft deletes a localization
func (s *Server) DeleteLocalization(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocalizationRequest],
) (*connect.Response[pb.DeleteLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("localization id is required"))
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("localization not found"))
	}

	// Extract user ID from context for history tracking
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		localization.UpdatedBy = &userID
	}

	if err := s.db.Delete(&localization).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete localization: %w", err))
	}

	return connect.NewResponse(&pb.DeleteLocalizationResponse{}), nil
}
