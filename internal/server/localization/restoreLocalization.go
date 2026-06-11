package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreLocalization(
	ctx context.Context,
	req *connect.Request[pb.RestoreLocalizationRequest],
) (*connect.Response[pb.RestoreLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("localization id is required"))
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Unscoped().Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("localization not found"))
	}

	localization.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&localization).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore localization: %w", err))
	}

	return connect.NewResponse(&pb.RestoreLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
