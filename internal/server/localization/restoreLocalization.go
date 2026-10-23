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

func (s *Server) RestoreLocalization(
	ctx context.Context,
	req *connect.Request[pb.RestoreLocalizationRequest],
) (*connect.Response[pb.RestoreLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	localization, err := common.Restore[models.Localization](ctx, s.db.DB, req.Msg.Id, errors.ErrLocalizationNotFound, "localization not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RestoreLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
