package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteLocale soft deletes a locale
func (s *Server) DeleteLocale(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocaleRequest],
) (*connect.Response[pb.DeleteLocaleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteLocale); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	// Prevent deleting the default locale
	if locale.IsDefault {
		return nil, errors.FailedPrecondition(errors.ErrLocaleDefaultDelete, "cannot delete the default locale")
	}

	if err := s.db.Delete(&locale).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete locale")
	}

	return connect.NewResponse(&pb.DeleteLocaleResponse{}), nil
}
