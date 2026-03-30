package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteLocale soft deletes a locale
func (s *Server) DeleteLocale(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocaleRequest],
) (*connect.Response[pb.DeleteLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Prevent deleting the default locale
	if locale.IsDefault {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot delete the default locale"))
	}

	if err := s.db.Delete(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete locale: %w", err))
	}

	return connect.NewResponse(&pb.DeleteLocaleResponse{}), nil
}
