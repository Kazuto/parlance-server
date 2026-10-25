package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
)

// UpdateUser updates an existing user
func (s *Server) UpdateUser(
	ctx context.Context,
	req *connect.Request[pb.UpdateUserRequest],
) (*connect.Response[pb.UpdateUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	if req.Msg.Email != nil && req.Msg.GetEmail() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "email is required")
	}

	if req.Msg.Name != nil && req.Msg.GetName() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	if req.Msg.Password != nil && req.Msg.GetPassword() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "password must not be empty")
	}

	user, err := common.FindByID[models.User](s.db.WithContext(ctx), req.Msg.Id, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	// As in AccountService, an API token may not change how its own user signs in.
	if user.ID == middleware.UserID(ctx) && (req.Msg.Password != nil || req.Msg.Email != nil && req.Msg.GetEmail() != user.Email) {
		if err := middleware.RequireSignIn(ctx); err != nil {
			return nil, err
		}
	}

	if req.Msg.Email != nil {
		user.Email = req.Msg.GetEmail()
	}

	if req.Msg.Name != nil {
		user.Name = req.Msg.GetName()
	}

	if req.Msg.Locale != nil {
		if locale := req.Msg.GetLocale(); locale != "" {
			user.Locale = &locale
		} else {
			user.Locale = nil
		}
	}

	if req.Msg.Password != nil {
		hashedPassword, err := auth.HashPassword(req.Msg.GetPassword())
		if err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to hash password")
		}

		user.PasswordHash = hashedPassword
	}

	if req.Msg.Avatar != nil {
		if len(req.Msg.Avatar) == 0 {
			if err := s.avatars.Remove(ctx, user.ID); err != nil {
				return nil, err
			}

			user.Avatar = nil
		} else {
			address, err := s.avatars.Save(ctx, user.ID, req.Msg.Avatar)
			if err != nil {
				return nil, err
			}

			user.Avatar = &address
		}
	}

	var roles []models.Role

	if req.Msg.Roles != nil {
		var err error

		roles, err = common.ResolveByIDs[models.Role](s.db.WithContext(ctx), req.Msg.Roles.GetIds(), errors.ErrRoleNotFound, "one or more roles do not exist")
		if err != nil {
			return nil, err
		}
	}

	// Save only adds many2many links, so a sent selection replaces them explicitly.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Roles").Save(&user).Error; err != nil {
			return err
		}
		if req.Msg.Roles != nil {
			return tx.Model(&user).Association("Roles").Replace(roles)
		}

		return nil
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update user")
	}

	// Load roles for response
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", user.ID).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load updated user")
	}

	return connect.NewResponse(&pb.UpdateUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
