package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteLocale soft deletes a locale
func (s *Server) DeleteLocale(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocaleRequest],
) (*connect.Response[pb.DeleteLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	locale, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	// Prevent deleting the default locale
	if locale.IsDefault {
		return nil, errors.FailedPrecondition(errors.ErrLocaleDefaultDelete, "cannot delete the default locale")
	}

	if err := common.DeleteRow(ctx, s.db.DB, &locale); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete locale")
	}

	return connect.NewResponse(&pb.DeleteLocaleResponse{}), nil
}
