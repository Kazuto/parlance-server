package locale

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestOneDefaultLocaleAtATime(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	server := &Server{db: db}

	create := func(code string, isDefault bool) string {
		t.Helper()

		res, err := server.CreateLocale(ctx, connect.NewRequest(&pb.CreateLocaleRequest{
			Code:      code,
			IsDefault: isDefault,
			Names:     &pb.StringMap{Values: map[string]string{"en": code}},
		}))
		if err != nil {
			t.Fatalf("CreateLocale(%s) error = %v", code, err)
		}

		return res.Msg.Locale.Id
	}
	defaults := func() []string {
		t.Helper()

		var codes []string

		db.Model(&models.Locale{}).Where("is_default = ?", true).Pluck("code", &codes)

		return codes
	}

	// Bootstrap may already have made a default; each step must leave exactly one.
	create("aa", true)

	if got := defaults(); len(got) != 1 || got[0] != "aa" {
		t.Fatalf("after creating a default: defaults = %v, want [aa]", got)
	}

	bb := create("bb", false)
	if _, err := server.SetDefaultLocale(ctx, connect.NewRequest(&pb.SetDefaultLocaleRequest{LocaleId: bb})); err != nil {
		t.Fatalf("SetDefaultLocale() error = %v", err)
	}

	if got := defaults(); len(got) != 1 || got[0] != "bb" {
		t.Fatalf("after setting a default: defaults = %v, want [bb]", got)
	}
	// Saving the current default again keeps it.
	isDefault := true
	if _, err := server.UpdateLocale(ctx, connect.NewRequest(&pb.UpdateLocaleRequest{Id: bb, IsDefault: &isDefault})); err != nil {
		t.Fatalf("UpdateLocale() error = %v", err)
	}

	if got := defaults(); len(got) != 1 || got[0] != "bb" {
		t.Fatalf("after re-saving the default: defaults = %v, want [bb]", got)
	}
}
