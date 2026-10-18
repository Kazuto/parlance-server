package definition

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

func (s *Server) CreateDefinition(
	ctx context.Context,
	req *connect.Request[pb.CreateDefinitionRequest],
) (*connect.Response[pb.CreateDefinitionResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateDefinition); err != nil {
		return nil, err
	}

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
	var terminology models.Terminology
	if err := s.db.First(&terminology, "id = ?", req.Msg.TerminologyId).Error; err != nil {
		return nil, errors.NotFound(errors.ErrTerminologyNotFound, "terminology not found")
	}

	// Verify locale exists
	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.LocaleId).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	// Check if definition already exists for this terminology/locale combination
	var existing models.Definition
	err := s.db.Where("terminology_id = ? AND locale_id = ?", req.Msg.TerminologyId, req.Msg.LocaleId).
		First(&existing).Error

	if err == nil {
		return nil, errors.AlreadyExists(errors.ErrDefinitionAlreadyExists, "definition already exists for this terminology and locale")
	}

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)

	definition := &models.Definition{
		TerminologyID: req.Msg.TerminologyId,
		LocaleID:      req.Msg.LocaleId,
		Translation:   req.Msg.Translation,
	}

	if userID != "" {
		definition.CreatedBy = &userID
	}

	if err := s.db.Create(definition).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create definition")
	}

	return connect.NewResponse(&pb.CreateDefinitionResponse{
		Definition: converter.DefinitionToProto(definition),
	}), nil
}
