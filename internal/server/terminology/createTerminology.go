package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) CreateTerminology(
	ctx context.Context,
	req *connect.Request[pb.CreateTerminologyRequest],
) (*connect.Response[pb.CreateTerminologyResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateTerminology); err != nil {
		return nil, err
	}

	if req.Msg.Term == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "term is required")
	}

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)

	terminology := &models.Terminology{
		Term:        req.Msg.Term,
		Description: req.Msg.Description,
	}

	if userID != "" {
		terminology.CreatedBy = &userID
	}

	if err := s.db.Create(terminology).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create terminology")
	}

	return connect.NewResponse(&pb.CreateTerminologyResponse{
		Terminology: converter.TerminologyToProto(terminology),
	}), nil
}
