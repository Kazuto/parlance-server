package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// RoleToProto converts a database Role model to protobuf
func RoleToProto(r *models.Role) *pb.Role {
	if r == nil {
		return nil
	}

	return &pb.Role{
		Id:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Permissions: PermissionsToProto(r.Permissions),
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
		DeletedAt:   formatDeletedAt(r.DeletedAt),
	}
}

func RolesToProto(roles []models.Role) []*pb.Role {
	result := make([]*pb.Role, len(roles))
	for i, role := range roles {
		result[i] = RoleToProto(&role)
	}

	return result
}
