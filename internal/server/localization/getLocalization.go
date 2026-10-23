package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetLocalization returns a single localization by ID
func (s *Server) GetLocalization(
	ctx context.Context,
	req *connect.Request[pb.GetLocalizationRequest],
) (*connect.Response[pb.GetLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	localization, err := common.FindByID[models.Localization](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocalizationNotFound, "localization not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
