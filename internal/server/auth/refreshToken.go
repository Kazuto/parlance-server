package auth

import (
	"context"
	"time"

	"connectrpc.com/connect"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// RefreshToken generates new tokens from a refresh token
func (s *Server) RefreshToken(
	ctx context.Context,
	req *connect.Request[pb.RefreshTokenRequest],
) (*connect.Response[pb.RefreshTokenResponse], error) {
	if req.Msg.RefreshToken == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "refresh token is required")
	}

	// Validate refresh token
	claims, err := authpkg.ValidateToken(req.Msg.RefreshToken, s.config.JWT.Secret)
	if err != nil {
		return nil, errors.Unauthenticated(errors.ErrUnauthenticated, "invalid refresh token")
	}

	// Get user
	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil, errors.NotFound(errors.ErrUserNotFound, "user not found")
	}

	// Generate new tokens
	accessExpiry, err := time.ParseDuration(s.config.JWT.Expiry)
	if err != nil {
		accessExpiry = 24 * time.Hour // Default to 24 hours
	}
	refreshExpiry := accessExpiry * 7

	tokens, err := authpkg.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to generate tokens")
	}

	return connect.NewResponse(&pb.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(&user),
	}), nil
}
