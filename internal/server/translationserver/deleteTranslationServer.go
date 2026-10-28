package translationserver

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteTranslationServer removes a translation server
func (s *Server) DeleteTranslationServer(
	ctx context.Context,
	req *connect.Request[pb.DeleteTranslationServerRequest],
) (*connect.Response[pb.DeleteTranslationServerResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation server id is required")
	}

	result := s.db.Delete(&models.TranslationServer{}, "id = ?", req.Msg.Id)
	if result.Error != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete translation server")
	}

	if result.RowsAffected == 0 {
		return nil, errors.NotFound(errors.ErrTranslationServerNotFound, "translation server not found")
	}

	return connect.NewResponse(&pb.DeleteTranslationServerResponse{}), nil
}
