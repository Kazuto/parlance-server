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

// DestroyLocale permanently deletes a soft-deleted locale
func (s *Server) DestroyLocale(
	ctx context.Context,
	req *connect.Request[pb.DestroyLocaleRequest],
) (*connect.Response[pb.DestroyLocaleResponse], error) {
	// Check if user has permission to permanently delete locales
	if err := middleware.RequirePermission(ctx, s.db, permissions.PermanentDeleteLocale); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	var locale models.Locale
	// Find the locale including soft-deleted ones
	if err := s.db.Unscoped().First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	// Only allow destroying already soft-deleted locales
	if locale.DeletedAt.Time.IsZero() {
		return nil, errors.FailedPrecondition(errors.ErrLocaleMustBeSoftDeleted, "locale must be soft-deleted before permanent deletion")
	}

	// Prevent destroying the default locale
	if locale.IsDefault {
		return nil, errors.FailedPrecondition(errors.ErrLocaleDefaultDestroy, "cannot destroy the default locale")
	}

	// Permanently delete the locale and all associated localizations
	if err := s.db.Unscoped().Delete(&locale).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to destroy locale")
	}

	// Also permanently delete all localizations for this locale
	if err := s.db.Unscoped().Where("locale_id = ?", req.Msg.Id).Delete(&models.Localization{}).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to destroy locale localizations")
	}

	return connect.NewResponse(&pb.DestroyLocaleResponse{}), nil
}
