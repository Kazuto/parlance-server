package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// RemoveScopeFromEntry removes a scope from an entry
func (s *Server) RemoveScopeFromEntry(
	ctx context.Context,
	req *connect.Request[pb.RemoveScopeFromEntryRequest],
) (*connect.Response[pb.RemoveScopeFromEntryResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	if req.Msg.ScopeId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	entry, err := common.FindByID[models.Entry](s.db.WithContext(ctx), req.Msg.EntryId, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	scope, err := common.FindByID[models.Scope](s.db.WithContext(ctx), req.Msg.ScopeId, errors.ErrScopeNotFound, "scope not found")
	if err != nil {
		return nil, err
	}

	// The entry's tags of the scope go with it.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("entry_id = ? AND scope_id = ?", entry.ID, scope.ID).Delete(&models.EntryTag{}).Error; err != nil {
			return err
		}

		return tx.Model(&entry).Association("Scopes").Delete(&scope)
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to remove scope from entry")
	}

	return connect.NewResponse(&pb.RemoveScopeFromEntryResponse{}), nil
}
