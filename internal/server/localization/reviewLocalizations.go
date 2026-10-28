package localization

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
)

// ReviewLocalizations approves or rejects translations
func (s *Server) ReviewLocalizations(
	ctx context.Context,
	req *connect.Request[pb.ReviewLocalizationsRequest],
) (*connect.Response[pb.ReviewLocalizationsResponse], error) {
	localizations, err := common.ReviewRows[models.Localization](ctx, s.db.DB, common.ReviewSelection{
		IDs:             req.Msg.LocalizationIds,
		ParentIDs:       req.Msg.EntryIds,
		LocaleIDs:       req.Msg.LocaleIds,
		ParentColumn:    "entry_id",
		ApprovedEvent:   models.EventTranslationsApproved,
		Webhooks:        true,
		SkipApproved:    req.Msg.SkipApproved,
		ConfirmOutdated: req.Msg.ConfirmOutdated,
	}, req.Msg.Decision, req.Msg.Comment)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ReviewLocalizationsResponse{
		Localizations: converter.LocalizationsToProto(localizations),
	}), nil
}
