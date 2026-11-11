package database_test

import (
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/gorm"
)

func TestEntryTagsStayWithinTheEntrysScopes(t *testing.T) {
	db := testutil.SetupPostgresDB(t)

	// Migrating again finds the constraints in place.
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("migrate again: %v", err)
	}

	scope, other := models.Scope{Name: "Mobile", Slug: "mobile"}, models.Scope{Name: "Web", Slug: "web"}
	entry := models.Entry{Key: "welcome"}

	for _, row := range []any{&scope, &other, &entry} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	tag, otherTag := models.Tag{ScopeID: scope.ID, Name: "onboarding"}, models.Tag{ScopeID: other.ID, Name: "checkout"}
	for _, row := range []any{&tag, &otherTag} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Create(&models.Tag{ScopeID: scope.ID, Name: "Onboarding"}).Error; err == nil {
		t.Error("a second tag named onboarding in another case was created")
	}

	// The entry isn't in the scope yet.
	if err := db.Create(&models.EntryTag{EntryID: entry.ID, TagID: tag.ID, ScopeID: scope.ID}).Error; err == nil {
		t.Fatal("tagged an entry outside the tag's scope")
	}

	if err := db.Model(&entry).Association("Scopes").Append([]models.Scope{scope, other}); err != nil {
		t.Fatal(err)
	}

	// The scope must be the tag's.
	if err := db.Create(&models.EntryTag{EntryID: entry.ID, TagID: tag.ID, ScopeID: other.ID}).Error; err == nil {
		t.Fatal("tagged an entry with a tag under another scope")
	}

	for _, link := range []models.EntryTag{
		{EntryID: entry.ID, TagID: tag.ID, ScopeID: scope.ID},
		{EntryID: entry.ID, TagID: otherTag.ID, ScopeID: other.ID},
	} {
		if err := db.Create(&link).Error; err != nil {
			t.Fatalf("tag entry: %v", err)
		}
	}

	var loaded models.Entry
	if err := db.Preload("Tags").First(&loaded, "id = ?", entry.ID).Error; err != nil {
		t.Fatalf("preload tags: %v", err)
	}

	if len(loaded.Tags) != 2 {
		t.Errorf("preloaded %d tags, want 2", len(loaded.Tags))
	}

	// Leaving a scope, as replacing the scopes does, drops its tags.
	if err := db.Model(&entry).Association("Scopes").Replace([]models.Scope{other}); err != nil {
		t.Fatal(err)
	}

	assertTagCount(t, db.Where("tag_id = ?", tag.ID), 0)
	assertTagCount(t, db.Where("tag_id = ?", otherTag.ID), 1)

	// Deleting a tag drops it from its entries.
	if err := db.Delete(&otherTag).Error; err != nil {
		t.Fatalf("delete tag: %v", err)
	}

	assertTagCount(t, db.Where("entry_id = ?", entry.ID), 0)
}

// assertTagCount checks how many entry tags the query matches
func assertTagCount(t *testing.T, query *gorm.DB, want int64) {
	t.Helper()

	var count int64
	if err := query.Model(&models.EntryTag{}).Count(&count).Error; err != nil {
		t.Fatalf("count entry tags: %v", err)
	}

	if count != want {
		t.Errorf("got %d entry tags, want %d", count, want)
	}
}
