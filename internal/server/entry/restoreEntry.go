package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreEntry(
	ctx context.Context,
	req *connect.Request[pb.RestoreEntryRequest],
) (*connect.Response[pb.RestoreEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	entry, err := common.Restore[models.Entry](ctx, s.db.Preload("Localizations").Preload("Scopes"), req.Msg.Id, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RestoreEntryResponse{
		Entry: converter.EntryToProto(&entry),
	}), nil
}
