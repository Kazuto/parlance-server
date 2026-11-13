package scope

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestCreateTagValidation(t *testing.T) {
	ctx, db := testutil.Setup(t)
	scope := insertScope(t, db)

	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).CreateTag,
		&pb.CreateTagRequest{Name: "onboarding"},
		&pb.CreateTagRequest{ScopeId: scope, Name: "  "},
		&pb.CreateTagRequest{ScopeId: scope, Name: strings.Repeat("x", maxTagNameLength+1)},
	)
}

func TestCreateTagNamesAreUniquePerScope(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	mobile, web := insertScope(t, db), insertScope(t, db)

	res, err := server.CreateTag(ctx, connect.NewRequest(&pb.CreateTagRequest{ScopeId: mobile, Name: " Onboarding "}))
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	if res.Msg.Tag.Name != "Onboarding" || res.Msg.Tag.ScopeId != mobile {
		t.Errorf("got tag %v, want Onboarding in the mobile scope", res.Msg.Tag)
	}

	_, err = server.CreateTag(ctx, connect.NewRequest(&pb.CreateTagRequest{ScopeId: mobile, Name: "onboarding"}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name in the scope: expected already exists, got %v", err)
	}

	if _, err := server.CreateTag(ctx, connect.NewRequest(&pb.CreateTagRequest{ScopeId: web, Name: "onboarding"})); err != nil {
		t.Errorf("same name in another scope: %v", err)
	}
}

func TestTagsOfUnknownOrDeletedScopesAreNotFound(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	scope := insertScope(t, db)
	tag := insertTag(t, db, scope, "release")
	testutil.Exec(t, db, "UPDATE scopes SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", scope)

	_, err := server.CreateTag(ctx, connect.NewRequest(&pb.CreateTagRequest{ScopeId: scope, Name: "new"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("create in deleted scope: expected not found, got %v", err)
	}

	_, err = server.CreateTag(ctx, connect.NewRequest(&pb.CreateTagRequest{ScopeId: uuid.NewString(), Name: "new"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("create in unknown scope: expected not found, got %v", err)
	}

	_, err = server.UpdateTag(ctx, connect.NewRequest(&pb.UpdateTagRequest{Id: tag, Name: proto.String("renamed")}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("update tag of deleted scope: expected not found, got %v", err)
	}

	_, err = server.DeleteTag(ctx, connect.NewRequest(&pb.DeleteTagRequest{Id: tag}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("delete tag of deleted scope: expected not found, got %v", err)
	}
}

func TestUpdateTagRenames(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	scope := insertScope(t, db)
	tag := insertTag(t, db, scope, "release")
	insertTag(t, db, scope, "taken")

	testutil.ExpectInvalidArgument(t, ctx, server.UpdateTag,
		&pb.UpdateTagRequest{Name: proto.String("x")},
		&pb.UpdateTagRequest{Id: tag, Name: proto.String(" ")},
	)

	_, err := server.UpdateTag(ctx, connect.NewRequest(&pb.UpdateTagRequest{Id: tag, Name: proto.String("Taken")}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("rename to a taken name: expected already exists, got %v", err)
	}

	// Its own name in another case is no conflict.
	res, err := server.UpdateTag(ctx, connect.NewRequest(&pb.UpdateTagRequest{Id: tag, Name: proto.String("Release")}))
	if err != nil {
		t.Fatalf("UpdateTag: %v", err)
	}

	if res.Msg.Tag.Name != "Release" {
		t.Errorf("got name %q, want Release", res.Msg.Tag.Name)
	}
}

func TestDeleteTagRemovesItFromEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	scope := insertScope(t, db)
	tag, other := insertTag(t, db, scope, "release"), insertTag(t, db, scope, "other")
	entry := insertEntry(t, db)
	tagEntry(t, db, entry, tag, scope)
	tagEntry(t, db, entry, other, scope)

	if _, err := server.DeleteTag(ctx, connect.NewRequest(&pb.DeleteTagRequest{Id: tag})); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}

	var tags, links int64

	db.Raw("SELECT COUNT(*) FROM tags WHERE id = ?", tag).Scan(&tags)
	db.Raw("SELECT COUNT(*) FROM entry_tags WHERE entry_id = ?", entry).Scan(&links)

	if tags != 0 || links != 1 {
		t.Errorf("got %d tags and %d links left, want 0 and 1", tags, links)
	}
}

func TestScopesListTagsByNameWithCounts(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}
	scope := insertScope(t, db)
	beta, alpha := insertTag(t, db, scope, "beta"), insertTag(t, db, scope, "alpha")
	live, deleted := insertEntry(t, db), insertEntry(t, db)
	tagEntry(t, db, live, alpha, scope)
	tagEntry(t, db, deleted, alpha, scope)
	testutil.Exec(t, db, "UPDATE entries SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", deleted)

	res, err := server.GetScope(ctx, connect.NewRequest(&pb.GetScopeRequest{Id: scope}))
	if err != nil {
		t.Fatalf("GetScope: %v", err)
	}

	tags := res.Msg.Scope.Tags
	if len(tags) != 2 || tags[0].Id != alpha || tags[1].Id != beta {
		t.Fatalf("got tags %v, want alpha then beta", tags)
	}

	if tags[0].EntryCount != 1 || tags[1].EntryCount != 0 {
		t.Errorf("got counts %d and %d, want 1 and 0", tags[0].EntryCount, tags[1].EntryCount)
	}

	list, err := server.ListScopes(ctx, connect.NewRequest(&pb.ListScopesRequest{}))
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}

	for _, listed := range list.Msg.Scopes {
		if listed.Id == scope && (len(listed.Tags) != 2 || listed.Tags[0].EntryCount != 1) {
			t.Errorf("listed tags %v, want alpha with 1 entry and beta", listed.Tags)
		}
	}
}

func insertScope(t *testing.T, db *database.DB) string {
	t.Helper()

	id := uuid.NewString()
	testutil.Exec(t, db, "INSERT INTO scopes (id, name, slug) VALUES (?, ?, ?)", id, id, id)

	return id
}

func insertTag(t *testing.T, db *database.DB, scopeID, name string) string {
	t.Helper()

	id := uuid.NewString()
	testutil.Exec(t, db, "INSERT INTO tags (id, scope_id, name) VALUES (?, ?, ?)", id, scopeID, name)

	return id
}

func insertEntry(t *testing.T, db *database.DB) string {
	t.Helper()

	id := uuid.NewString()
	testutil.Exec(t, db, "INSERT INTO entries (id, key) VALUES (?, ?)", id, "key."+id)

	return id
}

func tagEntry(t *testing.T, db *database.DB, entryID, tagID, scopeID string) {
	t.Helper()

	testutil.Exec(t, db, "INSERT INTO entry_scopes (id, entry_id, scope_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", uuid.NewString(), entryID, scopeID)
	testutil.Exec(t, db, "INSERT INTO entry_tags (entry_id, tag_id, scope_id) VALUES (?, ?, ?)", entryID, tagID, scopeID)
}
