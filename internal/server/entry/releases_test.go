package entry

import (
	"slices"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestUpdateEntriesReleasesValidation(t *testing.T) {
	ctx, db := testutil.Setup(t)

	tooMany := make([]string, maxBulkEntries+1)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}

	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateEntriesReleases,
		&pb.UpdateEntriesReleasesRequest{AddReleaseIds: []string{"release"}},
		&pb.UpdateEntriesReleasesRequest{EntryIds: []string{"entry"}},
		&pb.UpdateEntriesReleasesRequest{EntryIds: []string{"entry"}, AddReleaseIds: []string{"release"}, RemoveReleaseIds: []string{"release"}},
		&pb.UpdateEntriesReleasesRequest{EntryIds: tooMany, AddReleaseIds: []string{"release"}},
	)
}

func TestUpdateEntriesReleasesAddsAndRemoves(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	held, fresh := insertEntry(t, db), insertEntry(t, db)
	spring, summer := insertRelease(t, db, nil), insertRelease(t, db, nil)
	addToRelease(t, db, held, summer)

	msg := &pb.UpdateEntriesReleasesRequest{
		EntryIds:         []string{held, fresh},
		AddReleaseIds:    []string{spring},
		RemoveReleaseIds: []string{summer},
	}
	// Running it twice must not duplicate links.
	for range 2 {
		if _, err := server.UpdateEntriesReleases(ctx, connect.NewRequest(msg)); err != nil {
			t.Fatalf("UpdateEntriesReleases: %v", err)
		}
	}

	assertReleases(t, db, held, spring)
	assertReleases(t, db, fresh, spring)
}

func TestReleasesThatWentOutKeepTheirEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	entry, shipped := insertEntry(t, db), insertEntry(t, db)
	released := insertRelease(t, db, new(time.Now().Add(-time.Hour)))
	addToRelease(t, db, shipped, released)

	for name, msg := range map[string]*pb.UpdateEntriesReleasesRequest{
		"add":    {EntryIds: []string{entry}, AddReleaseIds: []string{released}},
		"remove": {EntryIds: []string{shipped}, RemoveReleaseIds: []string{released}},
	} {
		_, err := server.UpdateEntriesReleases(ctx, connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition || !hasCode(err, errors.ErrReleaseAlreadyReleased) {
			t.Errorf("%s: expected %s, got %v", name, errors.ErrReleaseAlreadyReleased, err)
		}
	}

	assertReleases(t, db, shipped, released)

	_, err := server.UpdateEntriesReleases(ctx, connect.NewRequest(&pb.UpdateEntriesReleasesRequest{EntryIds: []string{entry}, AddReleaseIds: []string{uuid.NewString()}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !hasCode(err, errors.ErrReleaseNotFound) {
		t.Errorf("unknown release: expected %s, got %v", errors.ErrReleaseNotFound, err)
	}

	_, err = server.UpdateEntriesReleases(ctx, connect.NewRequest(&pb.UpdateEntriesReleasesRequest{EntryIds: []string{uuid.NewString()}, RemoveReleaseIds: []string{released}}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown entry: expected not found, got %v", err)
	}
}

func TestEntryFormSetsPendingReleases(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	spring, summer := insertRelease(t, db, nil), insertPendingRelease(t, db, time.Now().Add(time.Hour))
	shipped := insertRelease(t, db, new(time.Now().Add(-time.Hour)))

	created, err := server.CreateEntry(ctx, connect.NewRequest(&pb.CreateEntryRequest{
		Key: "release.form." + uuid.NewString(), Releases: &pb.IdSelection{Ids: []string{spring, summer}},
	}))
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	id := created.Msg.Entry.Id

	// Pending releases by date, unscheduled ones last.
	if got := releaseIDs(created.Msg.Entry); !slices.Equal(got, []string{summer, spring}) {
		t.Errorf("created entry is in %v, want summer then spring", got)
	}

	// A release that went out keeps the entry, but the entry doesn't show it.
	addToRelease(t, db, id, shipped)

	updated, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: id, Releases: &pb.IdSelection{Ids: []string{summer}}}))
	if err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	if got := releaseIDs(updated.Msg.Entry); !slices.Equal(got, []string{summer}) {
		t.Errorf("updated entry is in %v, want summer", got)
	}

	assertReleases(t, db, id, summer, shipped)

	// Without releases in the request, they stay.
	got, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: id, Description: new("Releases stay")}))
	if err != nil || !slices.Equal(releaseIDs(got.Msg.Entry), []string{summer}) {
		t.Errorf("update without releases: %v, %v", got, err)
	}

	for name, call := range map[string]func() error{
		"create": func() error {
			_, err := server.CreateEntry(ctx, connect.NewRequest(&pb.CreateEntryRequest{Key: "release.form." + uuid.NewString(), Releases: &pb.IdSelection{Ids: []string{shipped}}}))

			return err
		},
		"update": func() error {
			_, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: id, Releases: &pb.IdSelection{Ids: []string{shipped}}}))

			return err
		},
	} {
		if err := call(); !hasCode(err, errors.ErrReleaseAlreadyReleased) {
			t.Errorf("%s with a release that went out: expected %s, got %v", name, errors.ErrReleaseAlreadyReleased, err)
		}
	}
}

func TestListEntriesFiltersByReleases(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	spring, summer := insertRelease(t, db, nil), insertRelease(t, db, nil)
	first, second, both := insertEntry(t, db), insertEntry(t, db), insertEntry(t, db)
	insertEntry(t, db)
	addToRelease(t, db, first, spring)
	addToRelease(t, db, second, summer)
	addToRelease(t, db, both, spring)
	addToRelease(t, db, both, summer)

	for name, tc := range map[string]struct{ releases, want []string }{
		"one release":  {[]string{spring}, []string{first, both}},
		"two releases": {[]string{spring, summer}, []string{first, second, both}},
	} {
		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{ReleaseIds: tc.releases, Summary: true}))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		got := make([]string, len(res.Msg.Entries))
		for i, entry := range res.Msg.Entries {
			got[i] = entry.Id
		}

		sort.Strings(got)
		sort.Strings(tc.want)

		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

func TestCountLiveEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	locale, gone := uuid.NewString(), uuid.NewString()
	testutil.Exec(t, db, "INSERT INTO locales (id, code) VALUES (?, ?), (?, ?)", locale, "live-"+locale[:8], gone, "gone-"+gone[:8])
	testutil.Exec(t, db, "UPDATE locales SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", gone)

	live, draft, held, deletedLocale, untranslated := insertEntry(t, db), insertEntry(t, db), insertEntry(t, db), insertEntry(t, db), insertEntry(t, db)
	due := insertEntry(t, db)

	for entry, text := range map[string]struct{ locale, status string }{
		live:          {locale, models.StatusApproved},
		draft:         {locale, models.StatusDraft},
		held:          {locale, models.StatusApproved},
		deletedLocale: {gone, models.StatusApproved},
		due:           {locale, models.StatusApproved},
	} {
		testutil.Exec(t, db, "INSERT INTO localizations (id, entry_id, locale_id, translation, status) VALUES (?, ?, ?, 'Text', ?)", uuid.NewString(), entry, text.locale, text.status)
	}

	// A pending release holds its entries back; one whose date passed doesn't.
	addToRelease(t, db, held, insertRelease(t, db, nil))
	addToRelease(t, db, due, insertPendingRelease(t, db, time.Now().Add(-time.Minute)))

	res, err := server.CountLiveEntries(ctx, connect.NewRequest(&pb.CountLiveEntriesRequest{
		EntryIds: []string{live, draft, held, deletedLocale, untranslated, due, live},
	}))
	if err != nil {
		t.Fatalf("CountLiveEntries: %v", err)
	}

	if res.Msg.Count != 2 {
		t.Errorf("counted %d live entries, want 2", res.Msg.Count)
	}
}

// insertRelease creates a release, released at releasedAt or pending without it
func insertRelease(t *testing.T, db *database.DB, releasedAt *time.Time) string {
	t.Helper()

	release := models.Release{Name: "Release " + uuid.NewString(), ReleasedAt: releasedAt}
	if err := db.Create(&release).Error; err != nil {
		t.Fatalf("insert release: %v", err)
	}

	return release.ID
}

// insertPendingRelease creates a pending release with a date. It's kept in UTC, as the
// server keeps them: SQLite compares times as text.
func insertPendingRelease(t *testing.T, db *database.DB, releaseAt time.Time) string {
	t.Helper()

	releaseAt = releaseAt.UTC()

	release := models.Release{Name: "Release " + uuid.NewString(), ReleaseAt: &releaseAt}
	if err := db.Create(&release).Error; err != nil {
		t.Fatalf("insert release: %v", err)
	}

	return release.ID
}

func addToRelease(t *testing.T, db *database.DB, entryID, releaseID string) {
	t.Helper()

	testutil.Exec(t, db, "INSERT INTO release_entries (release_id, entry_id) VALUES (?, ?)", releaseID, entryID)
}

func assertReleases(t *testing.T, db *database.DB, entryID string, want ...string) {
	t.Helper()

	var got []string
	if err := db.Raw("SELECT release_id FROM release_entries WHERE entry_id = ?", entryID).Scan(&got).Error; err != nil {
		t.Fatalf("load releases: %v", err)
	}

	sort.Strings(got)
	sort.Strings(want)

	if !slices.Equal(got, want) {
		t.Fatalf("entry %s: got releases %v, want %v", entryID, got, want)
	}
}

func releaseIDs(entry *pb.Entry) []string {
	ids := make([]string, len(entry.Releases))
	for i, release := range entry.Releases {
		ids[i] = release.Id
	}

	return ids
}
