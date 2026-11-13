package importservice

import (
	"reflect"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestParseJSONTranslations(t *testing.T) {
	want := map[string]string{"entries.create": "Create Entry", "entries.form.key": "Key", "common.cancel": "Cancel", "empty": ""}

	for _, content := range []string{
		`{"entries.create":"Create Entry","entries.form.key":"Key","common.cancel":"Cancel","empty":""}`,
		`{"entries":{"create":"Create Entry","form":{"key":"Key"}},"common":{"cancel":"Cancel"},"empty":""}`,
		`{"entries":{"create":"Create Entry","form.key":"Key"},"common.cancel":"Cancel","empty":""}`,
	} {
		got, err := parseJSONTranslations([]byte(content))
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, %v; want %v", content, got, err, want)
		}
	}
}

func TestImportJSONRejectsInvalidFilesBeforeDatabaseChanges(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db} // Invalid input must fail before the database changes.

	for _, content := range []string{
		`{"a.b":"one","a":{"b":"two"}}`,
		`{"a":{"b":"two"},"a.b":"one"}`,
		`{"key":"one","key":"two"}`,
		`{"a":{"key":"one","key":"two"}}`,
		`{"key":null}`, `{"key":42}`, `{"key":true}`, `{"key":[]}`,
		`{"": "value"}`, `null`, `[]`, `"text"`,
		`{"key":"value"} {}`, `{"key":`,
	} {
		_, err := server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{LocaleId: "locale", FileContent: []byte(content)}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: expected invalid argument, got %v", content, err)
		}
	}
}

func TestImportReportsKeysOfDeletedEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)

	locale := models.Locale{Code: "xx"}
	deleted := models.Entry{Key: "gone"}

	for _, row := range []any{&locale, &deleted} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Delete(&deleted).Error; err != nil {
		t.Fatal(err)
	}

	response, err := (&Server{db: db}).ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{
		LocaleId: locale.ID, FileContent: []byte(`{"gone":"back","new":"text"}`),
	}))
	if err != nil {
		t.Fatalf("ImportJson() error = %v", err)
	}

	result := response.Msg.Result
	if result.EntriesCreated != 1 || result.EntriesFailed != 1 || len(result.Errors) != 1 || result.Errors[0].Key != "gone" {
		t.Fatalf("expected the new entry created and the deleted one failed, got %v", result)
	}

	var count int64
	if err := db.Model(&models.Localization{}).Where("entry_id = ?", deleted.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("expected no translation of the deleted entry, got %d, %v", count, err)
	}
}
