package role

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteRole soft deletes a Role
func (s *Server) DeleteRole(
	ctx context.Context,
	req *connect.Request[pb.DeleteRoleRequest],
) (*connect.Response[pb.DeleteRoleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "role id is required")
	}

	role, err := common.FindByID[models.Role](s.db.WithContext(ctx), req.Msg.Id, errors.ErrRoleNotFound, "role not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &role); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete Role")
	}

	return connect.NewResponse(&pb.DeleteRoleResponse{}), nil
}
