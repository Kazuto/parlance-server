package definition

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

func (s *Server) UpdateDefinition(
	ctx context.Context,
	req *connect.Request[pb.UpdateDefinitionRequest],
) (*connect.Response[pb.UpdateDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	if req.Msg.Translation != nil && req.Msg.GetTranslation() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation is required")
	}

	definition, err := common.FindByID[models.Definition](s.db.WithContext(ctx), req.Msg.Id, errors.ErrDefinitionNotFound, "definition not found")
	if err != nil {
		return nil, err
	}

	// Extract user ID from context
	definition.UpdatedBy = middleware.UserIDPtr(ctx)

	// New text needs review again
	if req.Msg.Translation != nil && req.Msg.GetTranslation() != definition.Translation {
		definition.Translation = req.Msg.GetTranslation()
		definition.MarkDraft()
	}

	if err := s.db.Save(&definition).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update definition")
	}

	return connect.NewResponse(&pb.UpdateDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
