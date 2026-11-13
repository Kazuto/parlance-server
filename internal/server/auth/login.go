package auth

import (
	"context"
	stderrors "errors"

	"connectrpc.com/connect"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"

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

	failed := func(reason string) error {
		s.failures.report(ctx, req.Peer().Addr, req.Header(), webhook.LoginFailedData{
			Email:  req.Msg.Email,
			Method: webhook.LoginMethodPassword,
			Reason: reason,
		})

		return errors.Unauthenticated(errors.ErrInvalidCredentials, "invalid email or password")
	}

	// Find user
	var user models.User
	if err := s.db.WithContext(ctx).Preload("Roles.Permissions").Where("email = ?", req.Msg.Email).First(&user).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, failed(webhook.LoginReasonUnknownEmail)
		}

		return nil, errors.InternalCause(err, "failed to load the user")
	}

	// Check password
	if err := authpkg.CheckPassword(req.Msg.Password, user.PasswordHash); err != nil {
		return nil, failed(webhook.LoginReasonWrongPassword)
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
