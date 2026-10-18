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

// CreateEntry creates a new entry
func (s *Server) CreateEntry(
	ctx context.Context,
	req *connect.Request[pb.CreateEntryRequest],
) (*connect.Response[pb.CreateEntryResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateEntry); err != nil {
		return nil, err
	}

	if req.Msg.Key == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry key is required")
	}

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)

	entry := &models.Entry{
		Key:         req.Msg.Key,
		Description: req.Msg.Description,
	}

	if userID != "" {
		entry.CreatedBy = &userID
	}

	scopes, err := resolveScopes(s.db.WithContext(ctx), req.Msg.GetScopes().GetIds())
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		if len(scopes) > 0 {
			return tx.Model(entry).Association("Scopes").Append(scopes)
		}
		return nil
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create entry")
	}

	// Load relationships for response
	s.db.Preload("Localizations").Preload("Scopes").First(entry, "id = ?", entry.ID)

	return connect.NewResponse(&pb.CreateEntryResponse{
		Entry: converter.EntryToProto(entry, "en"),
	}), nil
}
