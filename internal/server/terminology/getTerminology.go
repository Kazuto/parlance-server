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

func (s *Server) GetTerminology(
	ctx context.Context,
	req *connect.Request[pb.GetTerminologyRequest],
) (*connect.Response[pb.GetTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	terminology, err := common.FindByID[models.Terminology](s.db.WithContext(ctx).Preload("Definitions"), req.Msg.Id, errors.ErrTerminologyNotFound, "terminology not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
