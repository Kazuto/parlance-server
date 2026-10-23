package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteLocalization soft deletes a localization
func (s *Server) DeleteLocalization(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocalizationRequest],
) (*connect.Response[pb.DeleteLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	localization, err := common.FindByID[models.Localization](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocalizationNotFound, "localization not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &localization); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete localization")
	}

	return connect.NewResponse(&pb.DeleteLocalizationResponse{}), nil
}
