package scope

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestListScopesSearchMatchesNameOrSlug(t *testing.T) {
	ctx, db := testutil.Setup(t)
	db.Exec("DELETE FROM scopes")

	for _, scope := range []models.Scope{
		{Name: "Mobile App", Slug: "mobile"},
		{Name: "Website", Slug: "web_shop"},
		{Name: "Shop", Slug: "webshop"},
	} {
		if err := db.Create(&scope).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}
	for search, want := range map[string][]string{
		"APP":  {"Mobile App"},
		"shop": {"Shop", "Website"},
		"web_": {"Website"}, // "_" matches itself, not any character
	} {
		res, err := server.ListScopes(ctx, connect.NewRequest(&pb.ListScopesRequest{Filter: &pb.FilterRequest{Search: search}}))
		if err != nil {
			t.Fatalf("%q: %v", search, err)
		}

		var got []string
		for _, scope := range res.Msg.Scopes {
			got = append(got, scope.Name)
		}

		if len(got) != len(want) || (len(got) > 0 && got[0] != want[0]) || res.Msg.Pagination.Total != int32(len(want)) {
			t.Errorf("%q: got %v (total %d), want %v", search, got, res.Msg.Pagination.Total, want)
		}
	}
}

func TestCreateScopeRejectsTakenSlug(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	taken := models.Scope{Name: "Taken", Slug: "taken-slug"}
	if err := db.Create(&taken).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Delete(&taken).Error; err != nil {
		t.Fatal(err)
	}

	_, err := server.CreateScope(ctx, connect.NewRequest(&pb.CreateScopeRequest{Name: "Again", Slug: "taken-slug"}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("got %v, want already exists", err)
	}
}
