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

func (s *Server) GetDefinition(
	ctx context.Context,
	req *connect.Request[pb.GetDefinitionRequest],
) (*connect.Response[pb.GetDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	definition, err := common.FindByID[models.Definition](s.db.WithContext(ctx), req.Msg.Id, errors.ErrDefinitionNotFound, "definition not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
