package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateEntry updates an existing entry
func (s *Server) UpdateEntry(
	ctx context.Context,
	req *connect.Request[pb.UpdateEntryRequest],
) (*connect.Response[pb.UpdateEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		entry.UpdatedBy = &userID
	}

	if req.Msg.Description != "" {
		entry.Description = req.Msg.Description
	}

	if err := s.db.Save(&entry).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update entry: %w", err))
	}

	// Load relationships for response
	s.db.Preload("Localizations").Preload("Scopes").First(&entry, "id = ?", entry.ID)

	return connect.NewResponse(&pb.UpdateEntryResponse{
		Entry: converter.EntryToProto(&entry, "en"),
	}), nil
}
