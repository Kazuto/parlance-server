package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateUser creates a new user
func (s *Server) CreateUser(
	ctx context.Context,
	req *connect.Request[pb.CreateUserRequest],
) (*connect.Response[pb.CreateUserResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateUser); err != nil {
		return nil, err
	}

	if req.Msg.Email == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "email is required")
	}

	if req.Msg.Password == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "password is required")
	}

	if req.Msg.Name == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	// Check if user already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Msg.Email).First(&existingUser).Error; err == nil {
		return nil, errors.AlreadyExists(errors.ErrUserAlreadyExists, "user with this email already exists")
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Msg.Password)
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to hash password")
	}

	user := &models.User{
		Email:        req.Msg.Email,
		PasswordHash: hashedPassword,
		Name:         req.Msg.Name,
	}

	if ids := req.Msg.GetRoles().GetIds(); len(ids) > 0 {
		roles, err := s.GetRoles(ids)
		if err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to get roles")
		}

		user.Roles = roles
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create user")
	}

	// Assign default viewer role
	var viewerRole models.Role
	if err := s.db.Where("name = ?", "viewer").First(&viewerRole).Error; err == nil {
		_ = s.db.Model(user).Association("Roles").Append(&viewerRole)
	}

	// Load roles for response
	s.db.Preload("Roles.Permissions").First(user, "id = ?", user.ID)

	return connect.NewResponse(&pb.CreateUserResponse{
		User: converter.UserToProto(user),
	}), nil
}
