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

// UpdateTag renames a tag
func (s *Server) UpdateTag(
	ctx context.Context,
	req *connect.Request[pb.UpdateTagRequest],
) (*connect.Response[pb.UpdateTagResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "tag id is required")
	}

	db := s.db.WithContext(ctx)

	tag, err := common.FindByID[models.Tag](common.ActiveTags(db), req.Msg.Id, errors.ErrTagNotFound, "tag not found")
	if err != nil {
		return nil, err
	}

	if req.Msg.Name != nil {
		name, err := tagName(req.Msg.GetName())
		if err != nil {
			return nil, err
		}

		if err := checkTagName(db, tag.ScopeID, name, tag.ID); err != nil {
			return nil, err
		}

		tag.Name = name
	}

	tag.UpdatedBy = middleware.UserIDPtr(ctx)

	if err := db.Save(&tag).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to update tag")
	}

	return connect.NewResponse(&pb.UpdateTagResponse{
		Tag: converter.TagToProto(&tag),
	}), nil
}
