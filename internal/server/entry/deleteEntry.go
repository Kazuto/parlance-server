package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteEntry soft deletes an entry
func (s *Server) DeleteEntry(
	ctx context.Context,
	req *connect.Request[pb.DeleteEntryRequest],
) (*connect.Response[pb.DeleteEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	entry, err := common.FindByID[models.Entry](s.db.WithContext(ctx), req.Msg.Id, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &entry); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete entry")
	}

	return connect.NewResponse(&pb.DeleteEntryResponse{}), nil
}
