package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ListDefinitions(
	ctx context.Context,
	req *connect.Request[pb.ListDefinitionsRequest],
) (*connect.Response[pb.ListDefinitionsResponse], error) {
	if req.Msg.TerminologyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
	}

	var definitions []models.Definition
	if err := s.db.Where("terminology_id = ?", req.Msg.TerminologyId).Find(&definitions).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoDefinitions := make([]*pb.Definition, len(definitions))
	for i, def := range definitions {
		protoDefinitions[i] = converter.DefinitionToProto(&def)
	}

	return connect.NewResponse(&pb.ListDefinitionsResponse{
		Definitions: protoDefinitions,
	}), nil
}
