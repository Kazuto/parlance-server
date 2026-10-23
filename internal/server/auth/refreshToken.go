package auth

import (
	"context"

	"connectrpc.com/connect"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

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
	claims, err := authpkg.ValidateRefreshToken(req.Msg.RefreshToken, s.config.JWT.Secret)
	if err != nil {
		return nil, errors.Unauthenticated(errors.ErrUnauthenticated, "invalid refresh token")
	}

	// Get user
	user, err := common.FindByID[models.User](s.db.WithContext(ctx).Preload("Roles.Permissions"), claims.UserID, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	tokens, err := s.issueTokens(&user)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(&user),
	}), nil
}
