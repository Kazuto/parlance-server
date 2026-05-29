package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

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

func PermissionsToProto(Permissions []models.Permission) []*pb.Permission {
	result := make([]*pb.Permission, len(Permissions))
	for i, Permission := range Permissions {
		result[i] = PermissionToProto(&Permission)
	}
	return result
}
