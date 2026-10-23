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

func (s *Server) CreateDefinition(
	ctx context.Context,
	req *connect.Request[pb.CreateDefinitionRequest],
) (*connect.Response[pb.CreateDefinitionResponse], error) {
	if req.Msg.TerminologyId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	if req.Msg.LocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	if req.Msg.Translation == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation is required")
	}

	// Verify terminology exists
	if _, err := common.FindByID[models.Terminology](s.db.WithContext(ctx), req.Msg.TerminologyId, errors.ErrTerminologyNotFound, "terminology not found"); err != nil {
		return nil, err
	}

	// Verify locale exists
	if _, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.LocaleId, errors.ErrLocaleNotFound, "locale not found"); err != nil {
		return nil, err
	}

	exists, err := common.Exists(s.db.Model(&models.Definition{}).
		Where("terminology_id = ? AND locale_id = ?", req.Msg.TerminologyId, req.Msg.LocaleId))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to check existing definition")
	}

	if exists {
		return nil, errors.AlreadyExists(errors.ErrDefinitionAlreadyExists, "definition already exists for this terminology and locale")
	}

	definition := &models.Definition{
		TerminologyID: req.Msg.TerminologyId,
		LocaleID:      req.Msg.LocaleId,
		Translation:   req.Msg.Translation,
		CreatedBy:     middleware.UserIDPtr(ctx),
	}

	if err := s.db.Create(definition).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create definition")
	}

	return connect.NewResponse(&pb.CreateDefinitionResponse{
		Definition: converter.DefinitionToProto(definition),
	}), nil
}
