package entry

import (
	"context"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestUpdateEntriesTagsValidation(t *testing.T) {
	ctx, db := testutil.Setup(t)

	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateEntriesTags,
		&pb.UpdateEntriesTagsRequest{AddTagIds: []string{"tag"}},
		&pb.UpdateEntriesTagsRequest{EntryIds: []string{"entry"}},
		&pb.UpdateEntriesTagsRequest{EntryIds: []string{"entry"}, AddTagIds: []string{"tag"}, RemoveTagIds: []string{"tag"}},
	)
}

func TestUpdateEntriesTagsAddsScopesAndRemovesTags(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	scoped, unscoped := insertEntry(t, db), insertEntry(t, db)
	mobile := insertScope(t, db)
	onboarding, settings, old := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, mobile, "settings"), insertTag(t, db, mobile, "old")
	linkScope(t, db, scoped, mobile)
	tagEntry(t, db, scoped, old, mobile)

	msg := &pb.UpdateEntriesTagsRequest{
		EntryIds:     []string{scoped, unscoped},
		AddTagIds:    []string{onboarding, settings},
		RemoveTagIds: []string{old},
	}
	// Running it twice must not duplicate links.
	for range 2 {
		if _, err := server.UpdateEntriesTags(ctx, connect.NewRequest(msg)); err != nil {
			t.Fatalf("UpdateEntriesTags: %v", err)
		}
	}

	assertTags(t, db, scoped, onboarding, settings)
	assertTags(t, db, unscoped, onboarding, settings)
	// Adding a tag adds its scope, once.
	assertScopes(t, db, unscoped, mobile)
}

func TestUpdateEntriesTagsRejectsUnknownAndDeletedTags(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	entry, scope := insertEntry(t, db), insertScope(t, db)
	hidden := insertTag(t, db, scope, "hidden")
	testutil.Exec(t, db, "UPDATE scopes SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", scope)

	for name, tagID := range map[string]string{"unknown tag": uuid.NewString(), "tag of deleted scope": hidden} {
		_, err := server.UpdateEntriesTags(ctx, connect.NewRequest(&pb.UpdateEntriesTagsRequest{EntryIds: []string{entry}, AddTagIds: []string{tagID}}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: expected invalid argument, got %v", name, err)
		}
	}

	_, err := server.UpdateEntriesTags(ctx, connect.NewRequest(&pb.UpdateEntriesTagsRequest{EntryIds: []string{uuid.NewString()}, RemoveTagIds: []string{hidden}}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown entry: expected not found, got %v", err)
	}
}

func TestCreateEntryTagsMustBelongToItsScopes(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	mobile, web := insertScope(t, db), insertScope(t, db)
	onboarding, checkout := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, web, "checkout")

	_, err := server.CreateEntry(ctx, connect.NewRequest(&pb.CreateEntryRequest{
		Key:    "outside",
		Scopes: &pb.IdSelection{Ids: []string{mobile}},
		Tags:   &pb.IdSelection{Ids: []string{onboarding, checkout}},
	}))
	if !hasCode(err, errors.ErrTagOutsideScopes) {
		t.Errorf("tag of another scope: expected %s, got %v", errors.ErrTagOutsideScopes, err)
	}

	res, err := server.CreateEntry(ctx, connect.NewRequest(&pb.CreateEntryRequest{
		Key:    "inside",
		Scopes: &pb.IdSelection{Ids: []string{mobile}},
		Tags:   &pb.IdSelection{Ids: []string{onboarding}},
	}))
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	if tags := res.Msg.Entry.Tags; len(tags) != 1 || tags[0].Id != onboarding {
		t.Errorf("got tags %v, want onboarding", tags)
	}
}

func TestUpdateEntryReplacesTagsAndDropsThoseOfRemovedScopes(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	entry := insertEntry(t, db)
	mobile, web := insertScope(t, db), insertScope(t, db)
	onboarding, settings, checkout := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, mobile, "settings"), insertTag(t, db, web, "checkout")
	linkScope(t, db, entry, mobile)
	linkScope(t, db, entry, web)
	tagEntry(t, db, entry, onboarding, mobile)
	tagEntry(t, db, entry, checkout, web)

	// Tags alone are checked against the scopes the entry has.
	res, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: entry, Tags: &pb.IdSelection{Ids: []string{settings, checkout}}}))
	if err != nil {
		t.Fatalf("UpdateEntry tags: %v", err)
	}

	if len(res.Msg.Entry.Tags) != 2 {
		t.Errorf("got tags %v, want checkout and settings", res.Msg.Entry.Tags)
	}

	assertTags(t, db, entry, settings, checkout)

	// Scopes alone drop the tags of scopes that go.
	if _, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: entry, Scopes: &pb.IdSelection{Ids: []string{mobile}}})); err != nil {
		t.Fatalf("UpdateEntry scopes: %v", err)
	}

	assertTags(t, db, entry, settings)

	// Tags with scopes are checked against the new scopes.
	_, err = server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{
		Id:     entry,
		Scopes: &pb.IdSelection{Ids: []string{web}},
		Tags:   &pb.IdSelection{Ids: []string{settings}},
	}))
	if !hasCode(err, errors.ErrTagOutsideScopes) {
		t.Errorf("tag of a removed scope: expected %s, got %v", errors.ErrTagOutsideScopes, err)
	}
}

func TestRemovingScopesDropsTheirTags(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	entry := insertEntry(t, db)
	mobile, web := insertScope(t, db), insertScope(t, db)
	onboarding, checkout := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, web, "checkout")
	linkScope(t, db, entry, mobile)
	linkScope(t, db, entry, web)
	tagEntry(t, db, entry, onboarding, mobile)
	tagEntry(t, db, entry, checkout, web)

	if _, err := server.UpdateEntriesScopes(ctx, connect.NewRequest(&pb.UpdateEntriesScopesRequest{EntryIds: []string{entry}, RemoveScopeIds: []string{web}})); err != nil {
		t.Fatalf("UpdateEntriesScopes: %v", err)
	}

	assertTags(t, db, entry, onboarding)
}

func TestEntryTagsHideThoseOfDeletedScopes(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	entry := insertEntry(t, db)
	mobile, web := insertScope(t, db), insertScope(t, db)
	onboarding, checkout := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, web, "checkout")
	tagEntry(t, db, entry, onboarding, mobile)
	tagEntry(t, db, entry, checkout, web)
	testutil.Exec(t, db, "UPDATE scopes SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", web)

	res, err := server.GetEntry(ctx, connect.NewRequest(&pb.GetEntryRequest{Id: entry}))
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}

	if tags := res.Msg.Entry.Tags; len(tags) != 1 || tags[0].Id != onboarding {
		t.Errorf("got tags %v, want only onboarding", tags)
	}
}

func TestListEntriesNarrowsScopesToTheirSelectedTags(t *testing.T) {
	ctx, db := testutil.Setup(t)
	checkTagFilter(t, ctx, db)
}

// checkTagFilter lists entries by scopes and tags, checking that a scope's selected tags
// narrow it
func checkTagFilter(t *testing.T, ctx context.Context, db *database.DB) {
	t.Helper()

	server := &Server{db: db}
	mobile, web := insertScope(t, db), insertScope(t, db)
	onboarding, settings := insertTag(t, db, mobile, "onboarding"), insertTag(t, db, mobile, "settings")

	entries := map[string]string{}
	for _, key := range []string{"mobile.onboarding", "mobile.settings", "mobile.untagged", "web.page", "unscoped"} {
		entries[key] = insertEntryWithKey(t, db, key)
	}

	linkScope(t, db, entries["mobile.untagged"], mobile)
	linkScope(t, db, entries["web.page"], web)
	tagEntry(t, db, entries["mobile.onboarding"], onboarding, mobile)
	tagEntry(t, db, entries["mobile.settings"], settings, mobile)

	list := func(scopeIDs, tagIDs []string) []string {
		t.Helper()

		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{ScopeIds: scopeIDs, TagIds: tagIDs, Summary: true}))
		if err != nil {
			t.Fatalf("ListEntries: %v", err)
		}

		var keys []string
		for _, entry := range res.Msg.Entries {
			keys = append(keys, entry.Key)
		}

		sort.Strings(keys)

		return keys
	}

	cases := map[string]struct {
		scopes, tags []string
		want         []string
	}{
		"scope":                 {[]string{mobile}, nil, []string{"mobile.onboarding", "mobile.settings", "mobile.untagged"}},
		"scope with tag":        {[]string{mobile}, []string{onboarding}, []string{"mobile.onboarding"}},
		"scope with both tags":  {[]string{mobile}, []string{onboarding, settings}, []string{"mobile.onboarding", "mobile.settings"}},
		"tag narrows its scope": {[]string{mobile, web}, []string{onboarding}, []string{"mobile.onboarding", "web.page"}},
		"tag alone":             {nil, []string{settings}, []string{"mobile.settings"}},
	}

	for name, tc := range cases {
		got := list(tc.scopes, tc.tags)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)

			continue
		}

		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", name, got, tc.want)

				break
			}
		}
	}
}

func insertEntryWithKey(t *testing.T, db *database.DB, key string) string {
	t.Helper()

	id := uuid.NewString()
	testutil.Exec(t, db, "INSERT INTO entries (id, key) VALUES (?, ?)", id, key)

	return id
}

func insertTag(t *testing.T, db *database.DB, scopeID, name string) string {
	t.Helper()

	id := uuid.NewString()
	testutil.Exec(t, db, "INSERT INTO tags (id, scope_id, name) VALUES (?, ?, ?)", id, scopeID, name)

	return id
}

// tagEntry tags the entry, adding the tag's scope to it first when needed
func tagEntry(t *testing.T, db *database.DB, entryID, tagID, scopeID string) {
	t.Helper()

	testutil.Exec(t, db, "INSERT INTO entry_scopes (id, entry_id, scope_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", uuid.NewString(), entryID, scopeID)
	testutil.Exec(t, db, "INSERT INTO entry_tags (entry_id, tag_id, scope_id) VALUES (?, ?, ?)", entryID, tagID, scopeID)
}

func assertTags(t *testing.T, db *database.DB, entryID string, want ...string) {
	t.Helper()

	var got []string
	if err := db.Raw("SELECT tag_id FROM entry_tags WHERE entry_id = ?", entryID).Scan(&got).Error; err != nil {
		t.Fatalf("load tags: %v", err)
	}

	sort.Strings(got)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("entry %s: got tags %v, want %v", entryID, got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("entry %s: got tags %v, want %v", entryID, got, want)
		}
	}
}

// hasCode tells whether err carries the error code, see errors.ErrorWithCode
func hasCode(err error, code string) bool {
	return err != nil && strings.Contains(err.Error(), "["+code+"]")
}
