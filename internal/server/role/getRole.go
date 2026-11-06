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

// GetRole returns a single Role by ID
func (s *Server) GetRole(
	ctx context.Context,
	req *connect.Request[pb.GetRoleRequest],
) (*connect.Response[pb.GetRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "role id is required")
	}

	role, err := common.FindByID[models.Role](s.db.WithContext(ctx).Preload("Permissions"), req.Msg.Id, errors.ErrRoleNotFound, "role not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetRoleResponse{
		Role: converter.RoleToProto(&role),
	}), nil
}
