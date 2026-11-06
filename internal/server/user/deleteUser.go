package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteUser soft deletes a user
func (s *Server) DeleteUser(
	ctx context.Context,
	req *connect.Request[pb.DeleteUserRequest],
) (*connect.Response[pb.DeleteUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	user, err := common.FindByID[models.User](s.db.WithContext(ctx), req.Msg.Id, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	// Their API tokens are revoked, and don't come back if the user is restored.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := common.DeleteRow(ctx, tx, &user); err != nil {
			return err
		}

		return tx.Where("user_id = ?", user.ID).Delete(&models.APIToken{}).Error
	})
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete user")
	}

	return connect.NewResponse(&pb.DeleteUserResponse{}), nil
}
