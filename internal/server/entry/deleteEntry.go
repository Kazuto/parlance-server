package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteEntry soft deletes an entry
func (s *Server) DeleteEntry(
	ctx context.Context,
	req *connect.Request[pb.DeleteEntryRequest],
) (*connect.Response[pb.DeleteEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	if err := s.db.Delete(&entry).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete entry: %w", err))
	}

	return connect.NewResponse(&pb.DeleteEntryResponse{}), nil
}
