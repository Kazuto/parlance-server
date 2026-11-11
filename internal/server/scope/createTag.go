package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateTag adds a tag to a scope
func (s *Server) CreateTag(
	ctx context.Context,
	req *connect.Request[pb.CreateTagRequest],
) (*connect.Response[pb.CreateTagResponse], error) {
	if req.Msg.ScopeId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "scope id is required")
	}

	name, err := tagName(req.Msg.Name)
	if err != nil {
		return nil, err
	}

	db := s.db.WithContext(ctx)

	scope, err := common.FindByID[models.Scope](db, req.Msg.ScopeId, errors.ErrScopeNotFound, "scope not found")
	if err != nil {
		return nil, err
	}

	if err := checkTagName(db, scope.ID, name, ""); err != nil {
		return nil, err
	}

	userID := middleware.UserIDPtr(ctx)
	tag := &models.Tag{ScopeID: scope.ID, Name: name, CreatedBy: userID, UpdatedBy: userID}

	if err := db.Create(tag).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create tag")
	}

	return connect.NewResponse(&pb.CreateTagResponse{
		Tag: converter.TagToProto(tag),
	}), nil
}
