package release

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
	"google.golang.org/protobuf/proto"
)

// emptyReleases starts from, and leaves, no releases, entries or announcements: the
// package's tests share a database.
func emptyReleases(t *testing.T, db *database.DB) {
	t.Helper()

	empty := func() {
		for _, table := range []string{"release_entries", "releases", "entries", "webhook_deliveries", "webhooks", "task_runs", "tasks"} {
			db.Exec("DELETE FROM " + table)
		}
	}
	empty()
	t.Cleanup(empty)
}

// insertRelease creates a release; a zero releasedAt leaves it pending
func insertRelease(t *testing.T, db *database.DB, name string, releaseAt *time.Time, releasedAt *time.Time) models.Release {
	t.Helper()

	release := models.Release{Name: name, ReleaseAt: releaseAt, ReleasedAt: releasedAt}
	if err := db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}

	return release
}

// insertEntries creates entries with the keys in the release, if any
func insertEntries(t *testing.T, db *database.DB, release *models.Release, keys ...string) []string {
	t.Helper()

	ids := make([]string, len(keys))

	for i, key := range keys {
		entry := models.Entry{Key: key}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}

		ids[i] = entry.ID

		if release != nil {
			if err := db.Create(&models.ReleaseEntry{ReleaseID: release.ID, EntryID: entry.ID}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	return ids
}

func at(t time.Time) *time.Time {
	return &t
}

func TestCreateReleaseValidation(t *testing.T) {
	ctx, db := testutil.Setup(t)
	emptyReleases(t, db)

	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).CreateRelease,
		&pb.CreateReleaseRequest{Name: "  "},
		&pb.CreateReleaseRequest{Name: strings.Repeat("x", maxReleaseNameLength+1)},
		&pb.CreateReleaseRequest{Name: "Spring", ReleaseAt: proto.String("tomorrow")},
	)
}

func TestReleaseNamesAreUnique(t *testing.T) {
	ctx, db := testutil.Setup(t)
	emptyReleases(t, db)

	server := &Server{db: db}

	res, err := server.CreateRelease(ctx, connect.NewRequest(&pb.CreateReleaseRequest{
		Name: " Spring launch ", Description: "New onboarding", ReleaseAt: proto.String("2030-04-01T09:00:00+02:00"),
	}))
	if err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}

	created := res.Msg.Release
	if created.Name != "Spring launch" || created.GetReleaseAt() != "2030-04-01T07:00:00Z" || created.ReleasedAt != nil {
		t.Errorf("created %v, want Spring launch at 07:00 UTC, pending", created)
	}

	_, err = server.CreateRelease(ctx, connect.NewRequest(&pb.CreateReleaseRequest{Name: "SPRING LAUNCH"}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name: expected already exists, got %v", err)
	}

	other := insertRelease(t, db, "Summer", nil, nil)

	_, err = server.UpdateRelease(ctx, connect.NewRequest(&pb.UpdateReleaseRequest{Id: other.ID, Name: proto.String("spring launch")}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("rename to a taken name: expected already exists, got %v", err)
	}

	// Its own name in another case is no clash.
	if _, err := server.UpdateRelease(ctx, connect.NewRequest(&pb.UpdateReleaseRequest{Id: other.ID, Name: proto.String("SUMMER")})); err != nil {
		t.Errorf("rename to its own name: %v", err)
	}
}

func TestReleaseDateOnlyMovesWhilePending(t *testing.T) {
	ctx, db := testutil.Setup(t)
	emptyReleases(t, db)

	server := &Server{db: db}
	release := insertRelease(t, db, "Spring", at(time.Now().Add(time.Hour)), nil)

	res, err := server.UpdateRelease(ctx, connect.NewRequest(&pb.UpdateReleaseRequest{Id: release.ID, ClearReleaseAt: true}))
	if err != nil || res.Msg.Release.ReleaseAt != nil {
		t.Fatalf("clear the date: %v, %v", res, err)
	}

	res, err = server.UpdateRelease(ctx, connect.NewRequest(&pb.UpdateReleaseRequest{Id: release.ID, ReleaseAt: proto.String("2030-01-02T03:04:05Z")}))
	if err != nil || res.Msg.Release.GetReleaseAt() != "2030-01-02T03:04:05Z" {
		t.Fatalf("set the date: %v, %v", res, err)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.UpdateRelease,
		&pb.UpdateReleaseRequest{Id: release.ID, ReleaseAt: proto.String("2030-01-02"), ClearReleaseAt: true},
		&pb.UpdateReleaseRequest{Id: release.ID, ReleaseAt: proto.String("soon")},
		&pb.UpdateReleaseRequest{Id: release.ID, Name: proto.String("")},
	)

	if _, err := server.PublishRelease(ctx, connect.NewRequest(&pb.PublishReleaseRequest{Id: release.ID})); err != nil {
		t.Fatalf("PublishRelease: %v", err)
	}

	for _, req := range []*pb.UpdateReleaseRequest{
		{Id: release.ID, ReleaseAt: proto.String("2031-01-01T00:00:00Z")},
		{Id: release.ID, ClearReleaseAt: true},
	} {
		if _, err := server.UpdateRelease(ctx, connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("%v after release: expected failed precondition, got %v", req, err)
		}
	}

	// The name and description still change.
	res, err = server.UpdateRelease(ctx, connect.NewRequest(&pb.UpdateReleaseRequest{
		Id: release.ID, Name: proto.String("Spring 2030"), Description: proto.String("Shipped"),
	}))
	if err != nil || res.Msg.Release.Name != "Spring 2030" || res.Msg.Release.Description != "Shipped" ||
		res.Msg.Release.GetReleaseAt() != "2030-01-02T03:04:05Z" || res.Msg.Release.ReleasedAt == nil {
		t.Errorf("rename after release: %v, %v", res, err)
	}
}

func TestListReleasesOrdersPendingByDateThenReleasedLatestFirst(t *testing.T) {
	ctx, db := testutil.Setup(t)
	emptyReleases(t, db)

	now := time.Now().UTC()
	later := insertRelease(t, db, "Later", at(now.Add(48*time.Hour)), nil)
	sooner := insertRelease(t, db, "Sooner", at(now.Add(24*time.Hour)), nil)
	insertRelease(t, db, "Unscheduled", nil, nil)
	// By name, Autumn would come first.
	insertRelease(t, db, "Autumn", at(now.Add(-72*time.Hour)), at(now.Add(-72*time.Hour)))
	insertRelease(t, db, "Winter", nil, at(now.Add(-time.Hour)))

	// Deleted entries don't count.
	ids := insertEntries(t, db, &sooner, "a", "b", "c")
	insertEntries(t, db, &later, "d")
	testutil.Exec(t, db, "UPDATE entries SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", ids[2])

	server := &Server{db: db}
	list := func(state pb.ReleaseState) []string {
		t.Helper()

		res, err := server.ListReleases(ctx, connect.NewRequest(&pb.ListReleasesRequest{State: state}))
		if err != nil {
			t.Fatalf("ListReleases(%v): %v", state, err)
		}

		names := make([]string, len(res.Msg.Releases))
		for i, release := range res.Msg.Releases {
			names[i] = release.Name

			if release.Name == "Sooner" && release.EntryCount != 2 {
				t.Errorf("Sooner counts %d entries, want 2", release.EntryCount)
			}
		}

		return names
	}

	for state, want := range map[pb.ReleaseState][]string{
		pb.ReleaseState_RELEASE_STATE_UNSPECIFIED: {"Sooner", "Later", "Unscheduled", "Winter", "Autumn"},
		pb.ReleaseState_RELEASE_STATE_PENDING:     {"Sooner", "Later", "Unscheduled"},
		pb.ReleaseState_RELEASE_STATE_RELEASED:    {"Winter", "Autumn"},
	} {
		if got := list(state); !slices.Equal(got, want) {
			t.Errorf("ListReleases(%v) = %v, want %v", state, got, want)
		}
	}

	testutil.ExpectInvalidArgument(t, ctx, server.ListReleases, &pb.ListReleasesRequest{State: pb.ReleaseState(9)})
}

func TestDeleteReleaseFreesItsEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)
	emptyReleases(t, db)

	server := &Server{db: db}
	release := insertRelease(t, db, "Spring", nil, nil)
	insertEntries(t, db, &release, "a", "b")

	if _, err := server.DeleteRelease(ctx, connect.NewRequest(&pb.DeleteReleaseRequest{Id: release.ID})); err != nil {
		t.Fatalf("DeleteRelease: %v", err)
	}

	var links int64

	db.Model(&models.ReleaseEntry{}).Count(&links)

	if links != 0 {
		t.Errorf("%d release links left, want none", links)
	}

	_, err := server.GetRelease(ctx, connect.NewRequest(&pb.GetReleaseRequest{Id: release.ID}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetRelease after delete: expected not found, got %v", err)
	}
}

func TestPublishReleaseAnnouncesOnce(t *testing.T) {
	ctx, db := testutil.Setup(t)
	emptyReleases(t, db)

	hook := models.Webhook{Name: "Launches", URL: "https://chat.example/hook", Enabled: true, Events: []string{models.WebhookEventTranslationsReleased}}
	task := models.Task{Name: "Export", Type: models.TaskTypeGitExport, Enabled: true, Events: []string{models.EventTranslationsReleased}}

	for _, row := range []any{&hook, &task} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}
	release := insertRelease(t, db, "Spring", at(time.Now().Add(time.Hour)), nil)
	other := insertRelease(t, db, "Summer", nil, nil)
	ids := insertEntries(t, db, &release, "onboarding.title", "onboarding.next", "pricing.title")

	// pricing.title waits for Summer too, so Spring doesn't let it out.
	if err := db.Create(&models.ReleaseEntry{ReleaseID: other.ID, EntryID: ids[2]}).Error; err != nil {
		t.Fatal(err)
	}

	res, err := server.PublishRelease(ctx, connect.NewRequest(&pb.PublishReleaseRequest{Id: release.ID}))
	if err != nil {
		t.Fatalf("PublishRelease: %v", err)
	}

	if res.Msg.Release.ReleasedAt == nil || res.Msg.Release.EntryCount != 3 {
		t.Errorf("published %v, want released with 3 entries", res.Msg.Release)
	}

	_, err = server.PublishRelease(ctx, connect.NewRequest(&pb.PublishReleaseRequest{Id: release.ID}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("publishing again: expected failed precondition, got %v", err)
	}

	var deliveries []models.WebhookDelivery

	db.Where("webhook_id = ?", hook.ID).Find(&deliveries)

	if len(deliveries) != 1 {
		t.Fatalf("%d deliveries, want 1", len(deliveries))
	}

	var body struct {
		Data webhook.ReleaseData `json:"data"`
	}

	if err := json.Unmarshal([]byte(deliveries[0].Payload), &body); err != nil {
		t.Fatalf("payload: %v", err)
	}

	keys := make([]string, len(body.Data.Entries))
	for i, entry := range body.Data.Entries {
		keys[i] = entry.Key
	}

	if body.Data.Release.Name != "Spring" || body.Data.Release.ReleasedAt == nil || body.Data.Count != 2 ||
		!slices.Equal(keys, []string{"onboarding.next", "onboarding.title"}) || body.Data.User == nil {
		t.Errorf("announced %+v, want Spring by the user with the two onboarding entries", body.Data)
	}

	var runs int64

	db.Model(&models.TaskRun{}).Where("task_id = ? AND trigger = ?", task.ID, models.TriggerEvent).Count(&runs)

	if runs != 1 {
		t.Errorf("%d task runs queued, want 1", runs)
	}
}
