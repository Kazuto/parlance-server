package translationserver

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListTranslationServers returns all translation servers, oldest first
func (s *Server) ListTranslationServers(
	ctx context.Context,
	req *connect.Request[pb.ListTranslationServersRequest],
) (*connect.Response[pb.ListTranslationServersResponse], error) {
	var servers []models.TranslationServer
	if err := s.db.Order("created_at").Find(&servers).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to list translation servers")
	}

	return connect.NewResponse(&pb.ListTranslationServersResponse{
		TranslationServers: converter.TranslationServersToProto(servers),
	}), nil
}
