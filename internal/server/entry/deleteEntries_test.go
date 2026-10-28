package entry

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestDeleteEntriesValidation(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	tooMany := make([]string, maxBulkEntries+1)

	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}

	testutil.ExpectInvalidArgument(t, ctx, server.DeleteEntries,
		&pb.DeleteEntriesRequest{},
		&pb.DeleteEntriesRequest{Ids: []string{"", ""}},
		&pb.DeleteEntriesRequest{Ids: tooMany},
	)
}

func TestDeleteEntriesSkipsWhatIsGone(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	first, second, kept := insertEntry(t, db), insertEntry(t, db), insertEntry(t, db)
	deleted := insertEntry(t, db)
	testutil.Exec(t, db, "UPDATE entries SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", deleted)

	res, err := server.DeleteEntries(ctx, connect.NewRequest(&pb.DeleteEntriesRequest{
		Ids: []string{first, second, first, deleted, uuid.NewString()},
	}))
	if err != nil {
		t.Fatalf("DeleteEntries: %v", err)
	}

	if res.Msg.DeletedCount != 2 {
		t.Errorf("deleted %d entries, want 2", res.Msg.DeletedCount)
	}

	for id, want := range map[string]bool{first: true, second: true, kept: false, deleted: true} {
		if got := isDeleted(t, db, id); got != want {
			t.Errorf("entry %s deleted = %v, want %v", id, got, want)
		}
	}
}

func isDeleted(t *testing.T, db *database.DB, id string) bool {
	t.Helper()

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM entries WHERE id = ? AND deleted_at IS NOT NULL", id).Scan(&count).Error; err != nil {
		t.Fatalf("check entry: %v", err)
	}

	return count == 1
}
