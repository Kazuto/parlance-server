package entry

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CountLiveEntries counts the entries exports carry with an approved text now, which a
// release would take out of exports
func (s *Server) CountLiveEntries(
	ctx context.Context,
	req *connect.Request[pb.CountLiveEntriesRequest],
) (*connect.Response[pb.CountLiveEntriesResponse], error) {
	entryIDs := common.UniqueIDs(req.Msg.EntryIds)

	if len(entryIDs) > maxBulkEntries {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many entries in one request")
	}

	var count int64

	if len(entryIDs) > 0 {
		approved := "EXISTS (SELECT 1 FROM localizations l JOIN locales ON locales.id = l.locale_id AND locales.deleted_at IS NULL" +
			" WHERE l.entry_id = entries.id AND l.status = ? AND l.deleted_at IS NULL)"

		if err := common.ExcludeHeldBack(s.db.WithContext(ctx).Model(&models.Entry{}), time.Now()).
			Where("entries.id IN ?", entryIDs).
			Where(approved, models.StatusApproved).
			Count(&count).Error; err != nil {
			return nil, errors.InternalCause(err, "failed to count live entries")
		}
	}

	return connect.NewResponse(&pb.CountLiveEntriesResponse{Count: int32(count)}), nil
}
