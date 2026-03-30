package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetLocalization returns a single localization by ID
func (s *Server) GetLocalization(
	ctx context.Context,
	req *connect.Request[pb.GetLocalizationRequest],
) (*connect.Response[pb.GetLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("localization id is required"))
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("localization not found"))
	}

	return connect.NewResponse(&pb.GetLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
