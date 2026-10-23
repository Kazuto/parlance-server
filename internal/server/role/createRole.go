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

// CreateRole creates a new Role
func (s *Server) CreateRole(
	ctx context.Context,
	req *connect.Request[pb.CreateRoleRequest],
) (*connect.Response[pb.CreateRoleResponse], error) {
	if req.Msg.Name == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	exists, err := common.Exists(s.db.Model(&models.Role{}).Where("name = ?", req.Msg.Name))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to check role name")
	}

	if exists {
		return nil, errors.AlreadyExists(errors.ErrRoleAlreadyExists, "role with this name already exists")
	}

	role := &models.Role{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
	}

	if ids := req.Msg.GetPermissions().GetIds(); len(ids) > 0 {
		permissions, err := common.ResolveByIDs[models.Permission](s.db.WithContext(ctx), ids, errors.ErrPermissionNotFound, "one or more permissions do not exist")
		if err != nil {
			return nil, err
		}

		role.Permissions = permissions
	}

	if err := s.db.Create(role).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create Role")
	}

	return connect.NewResponse(&pb.CreateRoleResponse{
		Role: converter.RoleToProto(role),
	}), nil
}
