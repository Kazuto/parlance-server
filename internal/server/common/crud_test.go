package common

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestFindByID(t *testing.T) {
	_, db := testutil.Setup(t)

	_, err := FindByID[models.Scope](db.DB, "missing", errors.ErrScopeNotFound, "scope not found")
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("missing row: got %v, want not found", err)
	}

	// A broken query is an internal error, not a missing row.
	_, err = FindByID[models.Scope](db.Table("no_such_table"), "missing", errors.ErrScopeNotFound, "scope not found")
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("failing query: got %v, want internal", err)
	}
}

func TestDeleteAndRestoreCreditTheCurrentUser(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	userID := middleware.UserID(ctx)

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	term := models.Terminology{Term: "greeting"}

	for _, row := range []any{&locale, &term} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	definition := models.Definition{TerminologyID: term.ID, LocaleID: locale.ID, Translation: "Gruß"}
	if err := db.Create(&definition).Error; err != nil {
		t.Fatal(err)
	}

	if err := DeleteRow(ctx, db.DB, &definition); err != nil {
		t.Fatalf("DeleteRow() error = %v", err)
	}

	restored, err := Restore[models.Definition](ctx, db.DB, definition.ID, errors.ErrDefinitionNotFound, "definition not found")
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	if restored.DeletedAt.Valid || restored.UpdatedBy == nil || *restored.UpdatedBy != userID {
		t.Errorf("restored row = %+v, want undeleted and updated by %s", restored, userID)
	}

	if _, err := FindByID[models.Definition](db.DB, definition.ID, errors.ErrDefinitionNotFound, "definition not found"); err != nil {
		t.Errorf("restored row not found: %v", err)
	}

	var history []models.DefinitionHistory
	if err := db.Where("definition_id = ?", definition.ID).Order("changed_at").Find(&history).Error; err != nil {
		t.Fatal(err)
	}

	if len(history) != 3 {
		t.Fatalf("history has %d rows, want created, deleted and updated", len(history))
	}

	for _, row := range history[1:] {
		if row.UserID == nil || *row.UserID != userID {
			t.Errorf("%s credited to %v, want %s", row.Action, row.UserID, userID)
		}
	}
}

func TestRestoreLeavesAssociationsAlone(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	userID := middleware.UserID(ctx)
	if err := db.Delete(&models.User{ID: userID}).Error; err != nil {
		t.Fatal(err)
	}

	user, err := Restore[models.User](ctx, db.Preload("Roles.Permissions"), userID, errors.ErrUserNotFound, "user not found")
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	if user.DeletedAt.Valid || len(user.Roles) != 1 || len(user.Roles[0].Permissions) != 1 {
		t.Errorf("restored user = %+v, want undeleted with its role", user)
	}

	var links int64

	db.Table("user_roles").Where("user_id = ?", userID).Count(&links)

	if links != 1 {
		t.Errorf("user has %d role links, want 1", links)
	}
}
