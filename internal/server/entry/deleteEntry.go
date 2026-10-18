package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteEntry soft deletes an entry
func (s *Server) DeleteEntry(
	ctx context.Context,
	req *connect.Request[pb.DeleteEntryRequest],
) (*connect.Response[pb.DeleteEntryResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteEntry); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrEntryNotFound, "entry not found")
	}

	if err := s.db.Delete(&entry).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete entry")
	}

	return connect.NewResponse(&pb.DeleteEntryResponse{}), nil
}
