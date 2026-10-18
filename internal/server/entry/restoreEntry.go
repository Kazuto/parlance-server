package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreEntry(
	ctx context.Context,
	req *connect.Request[pb.RestoreEntryRequest],
) (*connect.Response[pb.RestoreEntryResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteEntry); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	var entry models.Entry
	if err := s.db.Unscoped().Preload("Localizations").Preload("Scopes").First(&entry, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrEntryNotFound, "entry not found")
	}

	entry.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&entry).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to restore entry")
	}

	return connect.NewResponse(&pb.RestoreEntryResponse{
		Entry: converter.EntryToProto(&entry, "en"),
	}), nil
}
