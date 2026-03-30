package auth

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// Register creates a new user account
func (s *Server) Register(
	ctx context.Context,
	req *connect.Request[pb.RegisterRequest],
) (*connect.Response[pb.RegisterResponse], error) {
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
	hashedPassword, err := authpkg.HashPassword(req.Msg.Password)
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

	// Generate tokens
	accessExpiry, _ := time.ParseDuration(s.config.JWT.Expiry)
	refreshExpiry := accessExpiry * 7

	tokens, err := authpkg.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate tokens: %w", err))
	}

	return connect.NewResponse(&pb.RegisterResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(user),
	}), nil
}
