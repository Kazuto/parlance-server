package definition

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
)

// ReviewDefinitions approves or rejects definitions
func (s *Server) ReviewDefinitions(
	ctx context.Context,
	req *connect.Request[pb.ReviewDefinitionsRequest],
) (*connect.Response[pb.ReviewDefinitionsResponse], error) {
	definitions, err := common.ReviewRows[models.Definition](ctx, s.db.DB, common.ReviewSelection{
		IDs:          req.Msg.DefinitionIds,
		ParentIDs:    req.Msg.TerminologyIds,
		LocaleIDs:    req.Msg.LocaleIds,
		ParentColumn: "terminology_id",
	}, req.Msg.Decision, req.Msg.Comment)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ReviewDefinitionsResponse{
		Definitions: converter.DefinitionsToProto(definitions),
	}), nil
}
