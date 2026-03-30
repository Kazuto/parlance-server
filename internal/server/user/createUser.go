package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateUser creates a new user
func (s *Server) CreateUser(
	ctx context.Context,
	req *connect.Request[pb.CreateUserRequest],
) (*connect.Response[pb.CreateUserResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}

	if req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password is required"))
	}

	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	// Check if user already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Msg.Email).First(&existingUser).Error; err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("user with this email already exists"))
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password: %w", err))
	}

	user := &models.User{
		Email:        req.Msg.Email,
		PasswordHash: hashedPassword,
		Name:         req.Msg.Name,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user: %w", err))
	}

	// Assign default viewer role
	var viewerRole models.Role
	if err := s.db.Where("name = ?", "viewer").First(&viewerRole).Error; err == nil {
		s.db.Model(user).Association("Roles").Append(&viewerRole)
	}

	// Load roles for response
	s.db.Preload("Roles.Permissions").First(user, "id = ?", user.ID)

	return connect.NewResponse(&pb.CreateUserResponse{
		User: converter.UserToProto(user),
	}), nil
}
