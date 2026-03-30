package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateLocalization creates a new localization
func (s *Server) CreateLocalization(
	ctx context.Context,
	req *connect.Request[pb.CreateLocalizationRequest],
) (*connect.Response[pb.CreateLocalizationResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	if req.Msg.Translation == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("translation is required"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)

	localization := &models.Localization{
		EntryID:     req.Msg.EntryId,
		LocaleID:    req.Msg.LocaleId,
		Translation: req.Msg.Translation,
	}

	if userID != "" {
		localization.CreatedBy = &userID
	}

	if err := s.db.Create(localization).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create localization: %w", err))
	}

	return connect.NewResponse(&pb.CreateLocalizationResponse{
		Localization: converter.LocalizationToProto(localization),
	}), nil
}
