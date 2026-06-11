package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
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
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	var entry models.Entry
	if err := s.db.Preload("Localizations").Preload("Scopes").First(&entry, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetEntryResponse{
		Entry: converter.EntryToProto(&entry, requestLocale),
	}), nil
}
