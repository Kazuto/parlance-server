package auth

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/translators"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetCurrentUser returns the currently authenticated user
func (s *Server) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[pb.GetCurrentUserRequest],
) (*connect.Response[pb.GetCurrentUserResponse], error) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := ctx.Value(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return nil, errors.Unauthenticated(errors.ErrUnauthenticated, "not authenticated")
	}

	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
		return nil, errors.NotFound(errors.ErrUserNotFound, "user not found")
	}

	return connect.NewResponse(&pb.GetCurrentUserResponse{
		User:               converter.UserToProto(&user),
		TranslationEnabled: translators.Enabled(ctx, s.db),
	}), nil
}
