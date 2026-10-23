package entry

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
)

// UpdateEntry updates an existing entry
func (s *Server) UpdateEntry(
	ctx context.Context,
	req *connect.Request[pb.UpdateEntryRequest],
) (*connect.Response[pb.UpdateEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	if req.Msg.Key != nil && strings.TrimSpace(req.Msg.GetKey()) == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry key is required")
	}

	entry, err := common.FindByID[models.Entry](s.db.WithContext(ctx), req.Msg.Id, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	// Extract user ID from context
	entry.UpdatedBy = middleware.UserIDPtr(ctx)

	if req.Msg.Key != nil {
		key := strings.TrimSpace(req.Msg.GetKey())

		var count int64

		if err := s.db.WithContext(ctx).Model(&models.Entry{}).Where("key = ? AND id <> ?", key, entry.ID).Count(&count).Error; err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to check entry key")
		}

		if count > 0 {
			return nil, errors.AlreadyExists(errors.ErrEntryAlreadyExists, "entry key already exists")
		}

		entry.Key = key
	}

	if req.Msg.Description != nil {
		entry.Description = req.Msg.GetDescription()
	}

	var scopes []models.Scope

	if req.Msg.Scopes != nil {
		var err error

		scopes, err = common.ResolveByIDs[models.Scope](s.db.WithContext(ctx), req.Msg.Scopes.Ids, errors.ErrScopeNotFound, "one or more scopes do not exist")
		if err != nil {
			return nil, err
		}
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		if req.Msg.Scopes != nil {
			return tx.Model(&entry).Association("Scopes").Replace(scopes)
		}

		return nil
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update entry")
	}

	// Load relationships for response
	if err := s.db.Preload("Localizations").Preload("Scopes").First(&entry, "id = ?", entry.ID).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load updated entry")
	}

	return connect.NewResponse(&pb.UpdateEntryResponse{
		Entry: converter.EntryToProto(&entry),
	}), nil
}
