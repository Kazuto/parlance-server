package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// UserToProto converts a database User model to protobuf
func UserToProto(u *models.User) *pb.User {
	if u == nil {
		return nil
	}

	return &pb.User{
		Id:    u.ID,
		Email: u.Email,
		Name:  u.Name,
		// Locale stays unset: the column defaults to "en" for users who never chose one,
		// which would override the language the web app picked for them.
		Avatar:    derefString(u.Avatar),
		Roles:     RolesToProto(u.Roles),
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
		DeletedAt: formatDeletedAt(u.DeletedAt),
		// Accounts single sign-on created have no password hash.
		HasPassword: u.PasswordHash != "",
	}
}

func UsersToProto(users []models.User) []*pb.User {
	result := make([]*pb.User, len(users))
	for i, user := range users {
		result[i] = UserToProto(&user)
	}

	return result
}
