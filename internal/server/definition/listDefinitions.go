package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

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
	query := s.db.Where("terminology_id = ?", req.Msg.TerminologyId)

	allowedSorts := map[string]bool{"created_at": true, "created_by": true, "updated_at": true, "updated_by": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("created_at DESC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("translation LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Find(&definitions).Error; err != nil {
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
