package auth

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/translators"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetCurrentUser returns the currently authenticated user
func (s *Server) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[pb.GetCurrentUserRequest],
) (*connect.Response[pb.GetCurrentUserResponse], error) {
	// Extract user ID from context (set by auth middleware)
	userID := middleware.UserID(ctx)
	if userID == "" {
		return nil, errors.Unauthenticated(errors.ErrUnauthenticated, "not authenticated")
	}

	user, err := common.FindByID[models.User](s.db.WithContext(ctx).Preload("Roles.Permissions"), userID, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetCurrentUserResponse{
		User:               converter.UserToProto(&user),
		TranslationEnabled: translators.Enabled(ctx, s.db),
	}), nil
}
