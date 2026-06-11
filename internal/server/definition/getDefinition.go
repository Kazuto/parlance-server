package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) GetDefinition(
	ctx context.Context,
	req *connect.Request[pb.GetDefinitionRequest],
) (*connect.Response[pb.GetDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("definition id is required"))
	}

	var definition models.Definition
	if err := s.db.First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("definition not found"))
	}

	return connect.NewResponse(&pb.GetDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
