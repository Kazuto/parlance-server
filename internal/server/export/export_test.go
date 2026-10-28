package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestPhpString(t *testing.T) {
	for text, want := range map[string]string{
		"plain":     `'plain'`,
		"it's":      `'it\'s'`,
		`C:\path\`:  `'C:\\path\\'`,
		`a\'b`:      `'a\\\'b'`,
		"line\nend": "'line\nend'",
	} {
		if got := phpString(text); got != want {
			t.Errorf("phpString(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestExports(t *testing.T) {
	ctx, db := testutil.Setup(t)
	locale := models.Locale{Code: "ex", Names: datatypes.JSON(`{"en":"Export"}`)}
	translated, untranslated := models.Entry{Key: "ex.b.saved"}, models.Entry{Key: "ex.a.empty", Description: "No text yet"}
	markup := models.Entry{Key: "ex.c.markup"}

	for _, row := range []any{&locale, &translated, &untranslated, &markup} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Create(&models.Localization{
		EntryID: translated.ID, LocaleID: locale.ID, Translation: `It's in C:\temp\`,
		Review: models.Review{Status: models.StatusApproved},
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&models.Localization{
		EntryID: markup.ID, LocaleID: locale.ID, Translation: "Import & <export>",
		Review: models.Review{Status: models.StatusApproved},
	}).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}

	jsonRes, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "ex"}))
	if err != nil {
		t.Fatalf("ExportJson() error = %v", err)
	}

	// As people write such files: "&" and "<" as they are, and a final newline.
	wantJSON := "{\n  \"ex.b.saved\": \"It's in C:\\\\temp\\\\\",\n  \"ex.c.markup\": \"Import & <export>\"\n}\n"
	if jsonRes.Msg.Content != wantJSON {
		t.Errorf("JSON export = %q, want %q", jsonRes.Msg.Content, wantJSON)
	}

	// ZIP exports write their language files and manifest the same way.
	zipRes, err := server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{}))
	if err != nil {
		t.Fatalf("ExportZip() error = %v", err)
	}

	archive, err := zip.NewReader(bytes.NewReader(zipRes.Msg.Content), int64(len(zipRes.Msg.Content)))
	if err != nil {
		t.Fatalf("read ZIP: %v", err)
	}

	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}

		_ = reader.Close()

		if file.Name == "ex.json" && string(content) != wantJSON {
			t.Errorf("ZIP %s = %q, want %q", file.Name, content, wantJSON)
		}

		if !strings.HasSuffix(string(content), "}\n") || strings.HasSuffix(string(content), "\n\n") {
			t.Errorf("ZIP %s ends with %q, want one newline", file.Name, content[max(0, len(content)-3):])
		}
	}

	if jsonRes.Msg.Filename != "ex.json" {
		t.Errorf("JSON filename = %s", jsonRes.Msg.Filename)
	}

	phpRes, err := server.ExportPhp(ctx, connect.NewRequest(&pb.ExportPhpRequest{LocaleCode: "ex"}))
	if err != nil {
		t.Fatalf("ExportPhp() error = %v", err)
	}

	if want := "<?php\n\nreturn [\n    'ex.b.saved' => 'It\\'s in C:\\\\temp\\\\',\n    'ex.c.markup' => 'Import & <export>',\n];\n"; phpRes.Msg.Content != want {
		t.Errorf("PHP export = %q, want %q", phpRes.Msg.Content, want)
	}

	poRes, err := server.ExportPo(ctx, connect.NewRequest(&pb.ExportPoRequest{LocaleCode: "ex"}))
	if err != nil {
		t.Fatalf("ExportPo() error = %v", err)
	}
	// Untranslated entries stay in the catalog, ordered by key.
	if a, b := strings.Index(poRes.Msg.Content, "ex.a.empty"), strings.Index(poRes.Msg.Content, "ex.b.saved"); a < 0 || b < a {
		t.Errorf("PO export = %s", poRes.Msg.Content)
	}

	if _, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "nope"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown locale: got %v, want not found", err)
	}
}

func TestExportsLeaveOutDraftsUnlessIncluded(t *testing.T) {
	ctx, db := testutil.Setup(t)
	locale := models.Locale{Code: "dr", Names: datatypes.JSON(`{"en":"Drafts"}`)}
	approved, draft := models.Entry{Key: "dr.approved"}, models.Entry{Key: "dr.draft"}

	for _, row := range []any{&locale, &approved, &draft} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for entryID, status := range map[string]string{approved.ID: models.StatusApproved, draft.ID: models.StatusDraft} {
		if err := db.Create(&models.Localization{
			EntryID: entryID, LocaleID: locale.ID, Translation: "text",
			Review: models.Review{Status: status},
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}

	for includeDrafts, wantDraft := range map[bool]bool{false: false, true: true} {
		res, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "dr", IncludeDrafts: includeDrafts}))
		if err != nil {
			t.Fatalf("ExportJson() error = %v", err)
		}

		if !strings.Contains(res.Msg.Content, "dr.approved") || strings.Contains(res.Msg.Content, "dr.draft") != wantDraft {
			t.Errorf("include drafts %v: export = %s", includeDrafts, res.Msg.Content)
		}
	}
}
