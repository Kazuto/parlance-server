package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// RoleToProto converts a database Role model to protobuf
func RoleToProto(r *models.Role) *pb.Role {
	if r == nil {
		return nil
	}

	permissions := make([]*pb.Permission, len(r.Permissions))
	for i, perm := range r.Permissions {
		permissions[i] = PermissionToProto(&perm)
	}

	return &pb.Role{
		Id:          r.ID,
		Name:        r.Name,
		Permissions: permissions,
	}
}

func RolesToProto(roles []models.Role) []*pb.Role {
	result := make([]*pb.Role, len(roles))
	for i, role := range roles {
		result[i] = RoleToProto(&role)
	}
	return result
}
