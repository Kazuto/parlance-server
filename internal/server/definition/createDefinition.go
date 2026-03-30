package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) CreateDefinition(
	ctx context.Context,
	req *connect.Request[pb.CreateDefinitionRequest],
) (*connect.Response[pb.CreateDefinitionResponse], error) {
	if req.Msg.TerminologyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
	}

	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	if req.Msg.Translation == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("translation is required"))
	}

	// Verify terminology exists
	var terminology models.Terminology
	if err := s.db.First(&terminology, "id = ?", req.Msg.TerminologyId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("terminology not found"))
	}

	// Verify locale exists
	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.LocaleId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Check if definition already exists for this terminology/locale combination
	var existing models.Definition
	err := s.db.Where("terminology_id = ? AND locale_id = ?", req.Msg.TerminologyId, req.Msg.LocaleId).
		First(&existing).Error

	if err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("definition already exists for this terminology and locale"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)

	definition := &models.Definition{
		TerminologyID: req.Msg.TerminologyId,
		LocaleID:      req.Msg.LocaleId,
		Translation:   req.Msg.Translation,
	}

	if userID != "" {
		definition.CreatedBy = &userID
	}

	if err := s.db.Create(definition).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create definition: %w", err))
	}

	return connect.NewResponse(&pb.CreateDefinitionResponse{
		Definition: converter.DefinitionToProto(definition),
	}), nil
}
