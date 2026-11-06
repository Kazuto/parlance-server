package role

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreRole(
	ctx context.Context,
	req *connect.Request[pb.RestoreRoleRequest],
) (*connect.Response[pb.RestoreRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "role id is required")
	}

	role, err := common.Restore[models.Role](ctx, s.db.Preload("Permissions"), req.Msg.Id, errors.ErrRoleNotFound, "role not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RestoreRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
