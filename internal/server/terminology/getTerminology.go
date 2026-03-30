package terminology

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) GetTerminology(
	ctx context.Context,
	req *connect.Request[pb.GetTerminologyRequest],
) (*connect.Response[pb.GetTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
	}

	var terminology models.Terminology
	if err := s.db.Preload("Definitions").First(&terminology, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("terminology not found"))
	}

	return connect.NewResponse(&pb.GetTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
