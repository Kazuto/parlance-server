package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreEntry(
	ctx context.Context,
	req *connect.Request[pb.RestoreEntryRequest],
) (*connect.Response[pb.RestoreEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	var entry models.Entry
	if err := s.db.Preload("Localizations").Preload("Scopes").First(&entry, "id = ?", req.Msg.Id).Unscoped().Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	entry.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&entry).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore entry: %w", err))
	}

	return connect.NewResponse(&pb.RestoreEntryResponse{
		Entry: converter.EntryToProto(&entry, "en"),
	}), nil
}
