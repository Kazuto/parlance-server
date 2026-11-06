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

// GetEntry returns a single entry by ID
func (s *Server) GetEntry(
	ctx context.Context,
	req *connect.Request[pb.GetEntryRequest],
) (*connect.Response[pb.GetEntryResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	entry, err := common.FindByID[models.Entry](s.db.WithContext(ctx).Preload("Localizations").Preload("Scopes"), req.Msg.Id, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetEntryResponse{
		Entry: converter.EntryToProto(&entry),
	}), nil
}
