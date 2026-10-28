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

func (s *Server) CreateTerminology(
	ctx context.Context,
	req *connect.Request[pb.CreateTerminologyRequest],
) (*connect.Response[pb.CreateTerminologyResponse], error) {
	if req.Msg.Term == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "term is required")
	}

	// Terms stay unique across deleted terminologies too.
	exists, err := common.Exists(s.db.WithContext(ctx).Unscoped().Model(&models.Terminology{}).Where("term = ?", req.Msg.Term))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to check term")
	}

	if exists {
		return nil, errors.AlreadyExists(errors.ErrTerminologyAlreadyExists, "a terminology with this term already exists")
	}

	terminology := &models.Terminology{
		Term:        req.Msg.Term,
		Description: req.Msg.Description,
		CreatedBy:   middleware.UserIDPtr(ctx),
	}

	if err := s.db.Create(terminology).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create terminology")
	}

	return connect.NewResponse(&pb.CreateTerminologyResponse{
		Terminology: converter.TerminologyToProto(terminology),
	}), nil
}
