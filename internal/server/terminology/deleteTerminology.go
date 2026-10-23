package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) DeleteTerminology(
	ctx context.Context,
	req *connect.Request[pb.DeleteTerminologyRequest],
) (*connect.Response[pb.DeleteTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	terminology, err := common.FindByID[models.Terminology](s.db.WithContext(ctx), req.Msg.Id, errors.ErrTerminologyNotFound, "terminology not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &terminology); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete terminology")
	}

	return connect.NewResponse(&pb.DeleteTerminologyResponse{}), nil
}
