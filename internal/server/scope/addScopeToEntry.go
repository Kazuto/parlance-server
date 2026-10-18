package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// AddScopeToEntry adds a scope to an entry
func (s *Server) AddScopeToEntry(
	ctx context.Context,
	req *connect.Request[pb.AddScopeToEntryRequest],
) (*connect.Response[pb.AddScopeToEntryResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	if req.Msg.ScopeId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, errors.NotFound(errors.ErrEntryNotFound, "entry not found")
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.ScopeId).Error; err != nil {
		return nil, errors.NotFound(errors.ErrScopeNotFound, "scope not found")
	}

	if err := s.db.Model(&entry).Association("Scopes").Append(&scope); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to add scope to entry")
	}

	return connect.NewResponse(&pb.AddScopeToEntryResponse{}), nil
}
