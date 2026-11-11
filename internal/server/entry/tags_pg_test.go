package entry

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestListEntriesNarrowsScopesToTheirSelectedTagsInPostgres(t *testing.T) {
	checkTagFilter(t, context.Background(), testutil.SetupPostgresDB(t))
}

func TestTaggingEntriesInPostgres(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	ctx := context.Background()
	server := &Server{db: db}

	entry := insertEntry(t, db)
	mobile, web := insertScope(t, db), insertScope(t, db)
	onboarding, checkout := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, web, "checkout")

	// Two tags of different scopes the entry isn't in yet: both scopes are added.
	if _, err := server.UpdateEntriesTags(ctx, connect.NewRequest(&pb.UpdateEntriesTagsRequest{
		EntryIds:  []string{entry},
		AddTagIds: []string{onboarding, checkout},
	})); err != nil {
		t.Fatalf("UpdateEntriesTags: %v", err)
	}

	assertScopes(t, db, entry, mobile, web)
	assertTags(t, db, entry, onboarding, checkout)

	if _, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: entry, Scopes: &pb.IdSelection{Ids: []string{web}}})); err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	res, err := server.GetEntry(ctx, connect.NewRequest(&pb.GetEntryRequest{Id: entry}))
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}

	if tags := res.Msg.Entry.Tags; len(tags) != 1 || tags[0].Id != checkout {
		t.Errorf("got tags %v, want only checkout", tags)
	}
}
