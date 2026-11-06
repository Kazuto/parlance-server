package terminology

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

func (s *Server) UpdateTerminology(
	ctx context.Context,
	req *connect.Request[pb.UpdateTerminologyRequest],
) (*connect.Response[pb.UpdateTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	if req.Msg.Term != nil && req.Msg.GetTerm() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "term is required")
	}

	terminology, err := common.FindByID[models.Terminology](s.db.WithContext(ctx), req.Msg.Id, errors.ErrTerminologyNotFound, "terminology not found")
	if err != nil {
		return nil, err
	}

	// Extract user ID from context
	terminology.UpdatedBy = middleware.UserIDPtr(ctx)

	if req.Msg.Term != nil {
		terminology.Term = req.Msg.GetTerm()
	}

	if req.Msg.Description != nil {
		terminology.Description = req.Msg.GetDescription()
	}

	if err := s.db.Save(&terminology).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update terminology")
	}

	// Reload with definitions
	if err := s.db.Preload("Definitions").First(&terminology, "id = ?", terminology.ID).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to reload terminology")
	}

	return connect.NewResponse(&pb.UpdateTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
