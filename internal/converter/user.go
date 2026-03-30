package converter

import (
	"github.com/kazuto/parlance-server/internal/models"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UserToProto converts a database User model to protobuf
func UserToProto(u *models.User) *pb.User {
	if u == nil {
		return nil
	}

	roles := make([]*pb.Role, len(u.Roles))
	for i, role := range u.Roles {
		roles[i] = RoleToProto(&role)
	}

	return &pb.User{
		Id:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Roles:     roles,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

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

// PermissionToProto converts a database Permission model to protobuf
func PermissionToProto(p *models.Permission) *pb.Permission {
	if p == nil {
		return nil
	}

	return &pb.Permission{
		Id:       p.ID,
		Name:     p.Name,
		Resource: p.Resource,
		Action:   p.Action,
	}
}
