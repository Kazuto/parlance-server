package definition

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ListDefinitions(
	ctx context.Context,
	req *connect.Request[pb.ListDefinitionsRequest],
) (*connect.Response[pb.ListDefinitionsResponse], error) {
	if req.Msg.TerminologyId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	query := s.db.WithContext(ctx).Model(&models.Definition{}).Where("terminology_id = ?", req.Msg.TerminologyId)

	query, err := common.NewFilter(query, req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.DeleteDefinition).
		AllowedSorts(map[string]bool{"created_at": true, "created_by": true, "updated_at": true, "updated_by": true}).
		DefaultSort("created_at DESC").
		Search(common.SearchColumns("translation")).
		Apply()
	if err != nil {
		return nil, err
	}

	var definitions []models.Definition

	pagination, err := common.Paginate(query, req.Msg.Pagination, &definitions)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListDefinitionsResponse{
		Definitions: converter.DefinitionsToProto(definitions),
		Pagination:  pagination,
	}), nil
}
