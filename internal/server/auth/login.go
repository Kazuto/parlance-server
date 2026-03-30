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

// Login authenticates a user and returns tokens
func (s *Server) Login(
	ctx context.Context,
	req *connect.Request[pb.LoginRequest],
) (*connect.Response[pb.LoginResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}

	if req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password is required"))
	}

	// Find user
	var user models.User
	if err := s.db.Preload("Roles.Permissions").Where("email = ?", req.Msg.Email).First(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid email or password"))
	}

	// Check password
	if err := authpkg.CheckPassword(req.Msg.Password, user.PasswordHash); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid email or password"))
	}

	// Generate tokens
	accessExpiry, _ := time.ParseDuration(s.config.JWT.Expiry)
	refreshExpiry := accessExpiry * 7

	tokens, err := authpkg.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate tokens: %w", err))
	}

	return connect.NewResponse(&pb.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(&user),
	}), nil
}
