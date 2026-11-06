package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateScope creates a new scope
func (s *Server) CreateScope(
	ctx context.Context,
	req *connect.Request[pb.CreateScopeRequest],
) (*connect.Response[pb.CreateScopeResponse], error) {
	if req.Msg.Name == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope name is required")
	}

	if req.Msg.Slug == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope slug is required")
	}

	// Slugs stay unique across deleted scopes too.
	exists, err := common.Exists(s.db.WithContext(ctx).Unscoped().Model(&models.Scope{}).Where("slug = ?", req.Msg.Slug))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to check scope slug")
	}

	if exists {
		return nil, errors.AlreadyExists(errors.ErrScopeAlreadyExists, "a scope with this slug already exists")
	}

	scope := &models.Scope{
		Name:        req.Msg.Name,
		Slug:        req.Msg.Slug,
		Description: req.Msg.Description,
		Color:       req.Msg.Color,
	}

	if err := s.db.Create(scope).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create scope")
	}

	return connect.NewResponse(&pb.CreateScopeResponse{
		Scope: converter.ScopeToProto(scope),
	}), nil
}
