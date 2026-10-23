package export

import (
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

	for _, row := range []any{&locale, &translated, &untranslated} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Create(&models.Localization{EntryID: translated.ID, LocaleID: locale.ID, Translation: `It's in C:\temp\`}).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}

	jsonRes, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "ex"}))
	if err != nil {
		t.Fatalf("ExportJson() error = %v", err)
	}

	if !strings.Contains(jsonRes.Msg.Content, `"ex.b.saved": "It's in C:\\temp\\"`) || strings.Contains(jsonRes.Msg.Content, "ex.a.empty") {
		t.Errorf("JSON export = %s", jsonRes.Msg.Content)
	}

	if jsonRes.Msg.Filename != "ex.json" {
		t.Errorf("JSON filename = %s", jsonRes.Msg.Filename)
	}

	phpRes, err := server.ExportPhp(ctx, connect.NewRequest(&pb.ExportPhpRequest{LocaleCode: "ex"}))
	if err != nil {
		t.Fatalf("ExportPhp() error = %v", err)
	}

	if want := "<?php\n\nreturn [\n    'ex.b.saved' => 'It\\'s in C:\\\\temp\\\\',\n];\n"; phpRes.Msg.Content != want {
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
