// Package account serves the AccountService: the signed-in user's own account.
package account

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/avatar"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// Server implements the AccountService
type Server struct {
	db      *database.DB
	avatars *avatar.Store
}

// NewServer creates a new AccountService server
func NewServer(db *database.DB, avatars *avatar.Store) parlancev1connect.AccountServiceHandler {
	return &Server{db: db, avatars: avatars}
}

// UpdateAccount changes the given details of the signed-in user
func (s *Server) UpdateAccount(
	ctx context.Context,
	req *connect.Request[pb.UpdateAccountRequest],
) (*connect.Response[pb.UpdateAccountResponse], error) {
	user, err := s.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Msg.Name != nil {
		name := strings.TrimSpace(req.Msg.GetName())
		if name == "" {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
		}

		user.Name = name
	}

	if req.Msg.Email != nil {
		if user.Email, err = s.newEmail(ctx, &user, req.Msg.GetEmail()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Locale != nil {
		if locale := req.Msg.GetLocale(); locale != "" {
			user.Locale = &locale
		} else {
			user.Locale = nil
		}
	}

	if req.Msg.Avatar != nil {
		if err := s.setAvatar(ctx, &user, req.Msg.GetAvatar()); err != nil {
			return nil, err
		}
	}

	if err := s.db.WithContext(ctx).Omit("Roles").Save(&user).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to update the account")
	}

	if err := s.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, "id = ?", user.ID).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load the account")
	}

	return connect.NewResponse(&pb.UpdateAccountResponse{User: converter.UserToProto(&user)}), nil
}

// ChangePassword sets the signed-in user's password, checking the current one if
// there is one
func (s *Server) ChangePassword(
	ctx context.Context,
	req *connect.Request[pb.ChangePasswordRequest],
) (*connect.Response[pb.ChangePasswordResponse], error) {
	if req.Msg.NewPassword == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "the new password must not be empty")
	}

	user, err := s.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if user.PasswordHash != "" {
		if err := auth.CheckPassword(req.Msg.CurrentPassword, user.PasswordHash); err != nil {
			return nil, errors.InvalidInput(errors.ErrWrongPassword, "the current password is wrong")
		}
	}

	hash, err := auth.HashPassword(req.Msg.NewPassword)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to hash the password")
	}

	if err := s.db.WithContext(ctx).Model(&user).Update("password_hash", hash).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to change the password")
	}

	return connect.NewResponse(&pb.ChangePasswordResponse{}), nil
}

func (s *Server) currentUser(ctx context.Context) (models.User, error) {
	userID := middleware.UserID(ctx)
	if userID == "" {
		return models.User{}, errors.Unauthenticated(errors.ErrUnauthenticated, "user not authenticated")
	}

	return common.FindByID[models.User](s.db.WithContext(ctx), userID, errors.ErrUserNotFound, "user not found")
}

// newEmail checks an address the user switches to: given, and nobody else's,
// deleted users included, whatever its case.
func (s *Server) newEmail(ctx context.Context, user *models.User, value string) (string, error) {
	email := strings.TrimSpace(value)
	if email == "" || !strings.Contains(email, "@") {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "a valid email is required")
	}

	taken, err := common.Exists(s.db.WithContext(ctx).Unscoped().Model(&models.User{}).
		Where("LOWER(email) = LOWER(?) AND id <> ?", email, user.ID))
	if err != nil {
		return "", errors.InternalCause(err, "failed to check the email")
	}

	if taken {
		return "", errors.AlreadyExists(errors.ErrUserAlreadyExists, "another user has this email")
	}

	return email, nil
}

// setAvatar stores the image as the user's avatar, or removes it for an empty one.
func (s *Server) setAvatar(ctx context.Context, user *models.User, image []byte) error {
	if len(image) == 0 {
		user.Avatar = nil

		return s.avatars.Remove(ctx, user.ID)
	}

	address, err := s.avatars.Save(ctx, user.ID, image)
	if err != nil {
		return err
	}

	user.Avatar = &address

	return nil
}
