package stats

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestGetStatsCountsReviewStatusesPerLocale(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	testutil.Exec(t, db, `INSERT INTO locales (id, code, is_default) VALUES ('st-en', 'st-en', 1), ('st-de', 'st-de', 0)`)
	testutil.Exec(t, db, `INSERT INTO entries (id, key, deleted_at) VALUES
		('st-1', 'st.one', NULL), ('st-2', 'st.two', NULL), ('st-3', 'st.three', NULL),
		('st-4', 'st.deleted', '2026-01-01 00:00:00')`)
	testutil.Exec(t, db, `INSERT INTO localizations (id, entry_id, locale_id, translation, status) VALUES
		('st-l1', 'st-1', 'st-en', 'One', 'approved'),
		('st-l2', 'st-1', 'st-de', 'Eins', 'draft'),
		('st-l3', 'st-2', 'st-en', 'Two', 'approved'),
		('st-l4', 'st-2', 'st-de', 'Zwei', 'approved'),
		('st-l5', 'st-3', 'st-en', 'Three', 'draft'),
		('st-l6', 'st-4', 'st-de', 'Weg', 'approved')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localizations WHERE id LIKE 'st-%'`)
		db.Exec(`DELETE FROM entries WHERE id LIKE 'st-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'st-%'`)
	})

	res, err := server.GetStats(ctx, connect.NewRequest(&pb.GetStatsRequest{}))
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	type counts struct{ completed, approved, drafts, missing int32 }

	got := map[string]counts{}
	for _, locale := range res.Msg.ByLocale {
		got[locale.LocaleId] = counts{locale.Completed, locale.Approved, locale.Drafts, locale.Missing}
	}

	// The deleted entry's translation counts nowhere.
	want := map[string]counts{
		"st-en": {completed: 3, approved: 2, drafts: 1, missing: 0},
		"st-de": {completed: 2, approved: 1, drafts: 1, missing: 1},
	}

	for id, expected := range want {
		if got[id] != expected {
			t.Errorf("%s: got %+v, want %+v", id, got[id], expected)
		}
	}

	if res.Msg.DraftTranslations != 2 {
		t.Errorf("draft translations = %d, want 2", res.Msg.DraftTranslations)
	}
}
