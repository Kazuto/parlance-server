package localization

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateLocalization(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}
	server := &Server{db: db}

	insert := func(id, translation string) {
		t.Helper()
		err := db.Exec(
			`INSERT INTO localizations (id, entry_id, locale_id, translation) VALUES (?, 'ul-entry', 'ul-locale', ?)`,
			id, translation,
		).Error
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localizations WHERE id LIKE 'ul-%'`)
	})

	t.Run("rejects a missing id", func(t *testing.T) {
		_, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{
			Translation: proto.String("Hello"),
		}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("expected invalid argument, got %v", err)
		}
	})

	t.Run("reports an unknown localization", func(t *testing.T) {
		_, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{
			Id:          "ul-unknown",
			Translation: proto.String("Hello"),
		}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("updates the translation", func(t *testing.T) {
		insert("ul-update", "Hello")

		res, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{
			Id:          "ul-update",
			Translation: proto.String("Hello world"),
		}))
		if err != nil {
			t.Fatalf("UpdateLocalization: %v", err)
		}
		if got := res.Msg.Localization.GetTranslation(); got != "Hello world" {
			t.Errorf("response translation: got %q, want %q", got, "Hello world")
		}

		var stored models.Localization
		if err := db.First(&stored, "id = ?", "ul-update").Error; err != nil {
			t.Fatalf("load: %v", err)
		}
		if stored.Translation != "Hello world" {
			t.Errorf("stored translation: got %q, want %q", stored.Translation, "Hello world")
		}
	})

	// Clearing a translation in the editor sends an empty text, which removes it.
	t.Run("deletes the localization for an empty translation", func(t *testing.T) {
		insert("ul-clear", "Hello")

		res, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{
			Id:          "ul-clear",
			Translation: proto.String(""),
		}))
		if err != nil {
			t.Fatalf("UpdateLocalization: %v", err)
		}
		if res.Msg.Localization != nil {
			t.Errorf("expected no localization in the response, got %v", res.Msg.Localization)
		}

		var remaining int64
		db.Model(&models.Localization{}).Where("id = ?", "ul-clear").Count(&remaining)
		if remaining != 0 {
			t.Errorf("expected the localization to be deleted, %d left", remaining)
		}
		// Soft deleted, so its history and a restore stay possible.
		var deleted int64
		db.Unscoped().Model(&models.Localization{}).
			Where("id = ? AND deleted_at IS NOT NULL", "ul-clear").Count(&deleted)
		if deleted != 1 {
			t.Errorf("expected a soft-deleted row, found %d", deleted)
		}
	})
}
