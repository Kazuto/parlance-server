package entry

import (
	"sort"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestUpdateEntriesScopesValidation(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}
	cases := map[string]*pb.UpdateEntriesScopesRequest{
		"no entries":     {AddScopeIds: []string{"scope"}},
		"no scopes":      {EntryIds: []string{"entry"}},
		"add and remove": {EntryIds: []string{"entry"}, AddScopeIds: []string{"scope"}, RemoveScopeIds: []string{"scope"}},
	}
	for name, msg := range cases {
		_, err := server.UpdateEntriesScopes(ctx, connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: expected invalid argument, got %v", name, err)
		}
	}
}

func TestUpdateEntriesScopesAddsAndRemoves(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}
	first, second := insertEntry(t, db), insertEntry(t, db)
	keep, old, added := insertScope(t, db), insertScope(t, db), insertScope(t, db)
	linkScope(t, db, first, keep)
	linkScope(t, db, first, old)
	linkScope(t, db, second, added)

	msg := &pb.UpdateEntriesScopesRequest{
		EntryIds:       []string{first, second},
		AddScopeIds:    []string{added},
		RemoveScopeIds: []string{old},
	}
	// Running it twice must not duplicate links.
	for range 2 {
		if _, err := server.UpdateEntriesScopes(ctx, connect.NewRequest(msg)); err != nil {
			t.Fatalf("UpdateEntriesScopes: %v", err)
		}
	}

	assertScopes(t, db, first, keep, added)
	assertScopes(t, db, second, added)
}

func TestUpdateEntriesScopesRejectsUnknownIds(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}
	entry, scope := insertEntry(t, db), insertScope(t, db)
	deleted := insertEntry(t, db)
	db.Exec("UPDATE entries SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", deleted)

	cases := map[string]struct {
		msg  *pb.UpdateEntriesScopesRequest
		code connect.Code
	}{
		"unknown entry": {&pb.UpdateEntriesScopesRequest{EntryIds: []string{entry, uuid.NewString()}, AddScopeIds: []string{scope}}, connect.CodeNotFound},
		"deleted entry": {&pb.UpdateEntriesScopesRequest{EntryIds: []string{deleted}, AddScopeIds: []string{scope}}, connect.CodeNotFound},
		"unknown scope": {&pb.UpdateEntriesScopesRequest{EntryIds: []string{entry}, AddScopeIds: []string{uuid.NewString()}}, connect.CodeInvalidArgument},
	}
	for name, tc := range cases {
		_, err := server.UpdateEntriesScopes(ctx, connect.NewRequest(tc.msg))
		if connect.CodeOf(err) != tc.code {
			t.Errorf("%s: expected %v, got %v", name, tc.code, err)
		}
	}
	assertScopes(t, db, entry)
}

func insertEntry(t *testing.T, db *database.DB) string {
	t.Helper()
	id := uuid.NewString()
	if err := db.Exec("INSERT INTO entries (id, key) VALUES (?, ?)", id, "key."+id).Error; err != nil {
		t.Fatalf("insert entry: %v", err)
	}
	return id
}

func insertScope(t *testing.T, db *database.DB) string {
	t.Helper()
	id := uuid.NewString()
	if err := db.Exec("INSERT INTO scopes (id, name, slug) VALUES (?, ?, ?)", id, id, id).Error; err != nil {
		t.Fatalf("insert scope: %v", err)
	}
	return id
}

func linkScope(t *testing.T, db *database.DB, entryID, scopeID string) {
	t.Helper()
	if err := db.Exec("INSERT INTO entry_scopes (id, entry_id, scope_id) VALUES (?, ?, ?)", uuid.NewString(), entryID, scopeID).Error; err != nil {
		t.Fatalf("link scope: %v", err)
	}
}

func assertScopes(t *testing.T, db *database.DB, entryID string, want ...string) {
	t.Helper()
	var got []string
	if err := db.Raw("SELECT scope_id FROM entry_scopes WHERE entry_id = ?", entryID).Scan(&got).Error; err != nil {
		t.Fatalf("load scopes: %v", err)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("entry %s: got scopes %v, want %v", entryID, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("entry %s: got scopes %v, want %v", entryID, got, want)
		}
	}
}
