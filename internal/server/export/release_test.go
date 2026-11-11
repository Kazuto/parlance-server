package export

import (
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

// holdInRelease puts the entries with the keys in a new release. Times are kept in UTC,
// as the server keeps them: SQLite compares them as text.
func holdInRelease(t *testing.T, db *database.DB, releaseAt, releasedAt *time.Time, keys ...string) {
	t.Helper()

	for _, at := range []*time.Time{releaseAt, releasedAt} {
		if at != nil {
			*at = at.UTC()
		}
	}

	release := models.Release{Name: "Release of " + keys[0], ReleaseAt: releaseAt, ReleasedAt: releasedAt}
	if err := db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}

	for _, key := range keys {
		testutil.Exec(t, db, "INSERT INTO release_entries (release_id, entry_id) SELECT ?, id FROM entries WHERE key = ?", release.ID, key)
	}
}

func TestExportsHoldBackUnreleasedEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)
	f := seedSelection(t, db)
	server := &Server{db: db}

	hour := time.Hour
	future, past := new(time.Now().Add(hour)), new(time.Now().Add(-hour))

	// Held: a date to come, no date, and a pending release next to one that went out.
	holdInRelease(t, db, future, nil, "mail.receipt")
	holdInRelease(t, db, nil, nil, "web.home", "web.about")
	holdInRelease(t, db, new(*past), new(*past), "web.about", "mail.welcome")
	// Not held: the date came, though the runner hasn't let the release out yet.
	holdInRelease(t, db, new(*past), nil, "other")

	for name, tc := range map[string]struct {
		includeUnreleased bool
		want              []string
	}{
		"held back":      {false, []string{"mail.welcome", "other"}},
		"including held": {true, []string{"mail.receipt", "mail.welcome", "other", "web.about", "web.home"}},
	} {
		res, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "ex", IncludeUnreleased: tc.includeUnreleased}))
		if err != nil {
			t.Fatalf("%s: ExportJson() error = %v", name, err)
		}

		if got := jsonKeys(t, res.Msg.Content); !slices.Equal(got, tc.want) {
			t.Errorf("%s: keys = %v, want %v", name, got, tc.want)
		}

		zipped, err := server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{
			LocaleIds: []string{}, ScopeIds: []string{}, IncludeUnreleased: tc.includeUnreleased,
		}))
		if err != nil {
			t.Fatalf("%s: ExportZip() error = %v", name, err)
		}

		if got := jsonKeys(t, unzip(t, zipped.Msg.Content)["ex.json"]); !slices.Equal(got, tc.want) {
			t.Errorf("%s: ZIP keys = %v, want %v", name, got, tc.want)
		}
	}

	// Each scope's folder holds back too.
	zipped, err := server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{ScopeIds: []string{f.mail.ID}}))
	if err != nil {
		t.Fatalf("ExportZip() error = %v", err)
	}

	if mail := unzip(t, zipped.Msg.Content)["mail/ex.php"]; !strings.Contains(mail, "'mail.welcome'") || strings.Contains(mail, "mail.receipt") {
		t.Errorf("mail/ex.php = %s, want mail.welcome alone", mail)
	}
}
