package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
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

	deletedAt := ""
	if u.DeletedAt.Valid {
		deletedAt = u.DeletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &pb.User{
		Id:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Roles:     roles,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		DeletedAt: deletedAt,
	}
}

func UsersToProto(users []models.User) []*pb.User {
	result := make([]*pb.User, len(users))
	for i, user := range users {
		result[i] = UserToProto(&user)
	}
	return result
}
