package auth

import (
	"context"

	"connectrpc.com/connect"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// Login authenticates a user and returns tokens
func (s *Server) Login(
	ctx context.Context,
	req *connect.Request[pb.LoginRequest],
) (*connect.Response[pb.LoginResponse], error) {
	if req.Msg.Email == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "email is required")
	}

	if req.Msg.Password == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "password is required")
	}

	// Find user
	var user models.User
	if err := s.db.Preload("Roles.Permissions").Where("email = ?", req.Msg.Email).First(&user).Error; err != nil {
		return nil, errors.Unauthenticated(errors.ErrInvalidCredentials, "invalid email or password")
	}

	// Check password
	if err := authpkg.CheckPassword(req.Msg.Password, user.PasswordHash); err != nil {
		return nil, errors.Unauthenticated(errors.ErrInvalidCredentials, "invalid email or password")
	}

	tokens, err := s.issueTokens(&user)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(&user),
	}), nil
}
