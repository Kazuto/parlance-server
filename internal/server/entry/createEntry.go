package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateEntry creates a new entry
func (s *Server) CreateEntry(
	ctx context.Context,
	req *connect.Request[pb.CreateEntryRequest],
) (*connect.Response[pb.CreateEntryResponse], error) {
	if req.Msg.Key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry key is required"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)

	entry := &models.Entry{
		Key:         req.Msg.Key,
		Description: req.Msg.Description,
	}

	if userID != "" {
		entry.CreatedBy = &userID
	}

	if err := s.db.Create(entry).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create entry: %w", err))
	}

	// Associate scopes if provided
	if len(req.Msg.ScopeIds) > 0 {
		var scopes []models.Scope
		if err := s.db.Where("id IN ?", req.Msg.ScopeIds).Find(&scopes).Error; err == nil {
			if len(scopes) > 0 {
				s.db.Model(entry).Association("Scopes").Append(&scopes)
			}
		}
	}

	// Load relationships for response
	s.db.Preload("Localizations").Preload("Scopes").First(entry, "id = ?", entry.ID)

	return connect.NewResponse(&pb.CreateEntryResponse{
		Entry: converter.EntryToProto(entry, "en"),
	}), nil
}
