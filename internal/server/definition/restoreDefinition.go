package definition

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreDefinition(
	ctx context.Context,
	req *connect.Request[pb.RestoreDefinitionRequest],
) (*connect.Response[pb.RestoreDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	definition, err := common.Restore[models.Definition](ctx, s.db.DB, req.Msg.Id, errors.ErrDefinitionNotFound, "definition not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RestoreDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
