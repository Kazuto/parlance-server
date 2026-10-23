package definition

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) DeleteDefinition(
	ctx context.Context,
	req *connect.Request[pb.DeleteDefinitionRequest],
) (*connect.Response[pb.DeleteDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	definition, err := common.FindByID[models.Definition](s.db.WithContext(ctx), req.Msg.Id, errors.ErrDefinitionNotFound, "definition not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &definition); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete definition")
	}

	return connect.NewResponse(&pb.DeleteDefinitionResponse{}), nil
}
