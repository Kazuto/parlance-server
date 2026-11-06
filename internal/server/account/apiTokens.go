package account

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/apitoken"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// The limits of API tokens: how many a user may have, the length of their names, and
// the longest they may last
const (
	maxAPITokens        = 50
	maxAPITokenName     = 100
	maxAPITokenLifetime = 3650
)

// ListApiTokens returns the signed-in user's API tokens, newest first
func (s *Server) ListApiTokens(
	ctx context.Context,
	req *connect.Request[pb.ListApiTokensRequest],
) (*connect.Response[pb.ListApiTokensResponse], error) {
	userID, err := tokenOwner(ctx)
	if err != nil {
		return nil, err
	}

	var tokens []models.APIToken
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to list API tokens")
	}

	response := &pb.ListApiTokensResponse{Tokens: make([]*pb.ApiToken, len(tokens))}
	for i := range tokens {
		response.Tokens[i] = converter.APITokenToProto(&tokens[i])
	}

	return connect.NewResponse(response), nil
}

// CreateApiToken creates an API token for the signed-in user, and returns it this once
func (s *Server) CreateApiToken(
	ctx context.Context,
	req *connect.Request[pb.CreateApiTokenRequest],
) (*connect.Response[pb.CreateApiTokenResponse], error) {
	userID, err := tokenOwner(ctx)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Msg.Name)
	if name == "" || utf8.RuneCountInString(name) > maxAPITokenName {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "a name of up to 100 characters is required")
	}

	days := req.Msg.ExpiresInDays
	if days < 0 || days > maxAPITokenLifetime {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "expires_in_days must be from 0 (never) to 3650")
	}

	db := s.db.WithContext(ctx)

	var count int64
	if err := db.Model(&models.APIToken{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to count API tokens")
	}

	if count >= maxAPITokens {
		return nil, errors.FailedPrecondition(errors.ErrAPITokenLimit, "revoke an API token before creating another")
	}

	secret, hash, shown, err := apitoken.Generate()
	if err != nil {
		return nil, errors.InternalCause(err, "failed to generate an API token")
	}

	token := models.APIToken{UserID: userID, Name: name, TokenHash: hash, Prefix: shown}

	if days > 0 {
		expires := time.Now().AddDate(0, 0, int(days))
		token.ExpiresAt = &expires
	}

	if err := db.Create(&token).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create the API token")
	}

	return connect.NewResponse(&pb.CreateApiTokenResponse{
		Token:  converter.APITokenToProto(&token),
		Secret: secret,
	}), nil
}

// DeleteApiToken revokes one of the signed-in user's API tokens
func (s *Server) DeleteApiToken(
	ctx context.Context,
	req *connect.Request[pb.DeleteApiTokenRequest],
) (*connect.Response[pb.DeleteApiTokenResponse], error) {
	userID, err := tokenOwner(ctx)
	if err != nil {
		return nil, err
	}

	// Another user's token is as unknown as a missing one.
	token, err := common.FindByID[models.APIToken](s.db.WithContext(ctx).Where("user_id = ?", userID),
		req.Msg.Id, errors.ErrAPITokenNotFound, "API token not found")
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Delete(&token).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to revoke the API token")
	}

	return connect.NewResponse(&pb.DeleteApiTokenResponse{}), nil
}

// tokenOwner returns the signed-in user, who manages their API tokens; a request
// made with a token may not
func tokenOwner(ctx context.Context) (string, error) {
	if err := middleware.RequireSignIn(ctx); err != nil {
		return "", err
	}

	userID := middleware.UserID(ctx)
	if userID == "" {
		return "", errors.Unauthenticated(errors.ErrUnauthenticated, "user not authenticated")
	}

	return userID, nil
}
