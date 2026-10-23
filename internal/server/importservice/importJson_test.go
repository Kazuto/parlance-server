package importservice

import (
	"reflect"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
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
