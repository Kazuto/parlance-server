package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreTerminology(
	ctx context.Context,
	req *connect.Request[pb.RestoreTerminologyRequest],
) (*connect.Response[pb.RestoreTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	terminology, err := common.Restore[models.Terminology](ctx, s.db.Preload("Definitions"), req.Msg.Id, errors.ErrTerminologyNotFound, "terminology not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RestoreTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
