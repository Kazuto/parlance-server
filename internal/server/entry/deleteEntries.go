package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteEntries soft deletes several entries at once. Entries already deleted, or that
// don't exist, are skipped, so a list someone else changed meanwhile still goes through.
func (s *Server) DeleteEntries(
	ctx context.Context,
	req *connect.Request[pb.DeleteEntriesRequest],
) (*connect.Response[pb.DeleteEntriesResponse], error) {
	ids := common.UniqueIDs(req.Msg.Ids)

	if len(ids) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "at least one entry id is required")
	}

	if len(ids) > maxBulkEntries {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many entries in one request")
	}

	result := s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&models.Entry{})
	if result.Error != nil {
		return nil, errors.InternalCause(result.Error, "failed to delete entries")
	}

	return connect.NewResponse(&pb.DeleteEntriesResponse{DeletedCount: int32(result.RowsAffected)}), nil
}
