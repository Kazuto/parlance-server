package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DestroyLocale permanently deletes a soft-deleted locale
func (s *Server) DestroyLocale(
	ctx context.Context,
	req *connect.Request[pb.DestroyLocaleRequest],
) (*connect.Response[pb.DestroyLocaleResponse], error) {
	// Check if user has permission to hard delete locales
	if !middleware.HasPermission(ctx, s.db, "hard_delete_locale") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions to permanently delete locales"))
	}

	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	// Find the locale including soft-deleted ones
	if err := s.db.Unscoped().First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Only allow destroying already soft-deleted locales
	if locale.DeletedAt.Time.IsZero() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("locale must be soft-deleted before permanent deletion"))
	}

	// Prevent destroying the default locale
	if locale.IsDefault {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot destroy the default locale"))
	}

	// Permanently delete the locale and all associated localizations
	if err := s.db.Unscoped().Delete(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to destroy locale: %w", err))
	}

	// Also permanently delete all localizations for this locale
	if err := s.db.Unscoped().Where("locale_id = ?", req.Msg.Id).Delete(&models.Localization{}).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to destroy locale localizations: %w", err))
	}

	return connect.NewResponse(&pb.DestroyLocaleResponse{}), nil
}
