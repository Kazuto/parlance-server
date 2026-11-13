package export

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestFailedDownloadsAreReported(t *testing.T) {
	ctx, db := testutil.Setup(t)

	locale := models.Locale{Code: "fx", Names: datatypes.JSON(`{"en":"Failing"}`)}
	hook := models.Webhook{Name: "Alerts", URL: "https://chat.example/hook", Enabled: true, Events: []string{models.WebhookEventExportFailed}}

	for _, row := range []any{&locale, &hook} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries WHERE webhook_id = ?`, hook.ID)
		db.Exec(`DELETE FROM webhooks WHERE id = ?`, hook.ID)
		db.Exec(`DELETE FROM locales WHERE id = ?`, locale.ID)
	})

	sent := func() []webhook.ExportFailedData {
		t.Helper()

		var deliveries []models.WebhookDelivery

		db.Where("webhook_id = ?", hook.ID).Order("created_at").Find(&deliveries)

		result := make([]webhook.ExportFailedData, len(deliveries))

		for i, delivery := range deliveries {
			var body struct {
				Data webhook.ExportFailedData `json:"data"`
			}

			if err := json.Unmarshal([]byte(delivery.Payload), &body); err != nil {
				t.Fatalf("payload: %v", err)
			}

			result[i] = body.Data
		}

		return result
	}

	server := &Server{db: db}

	// Bad requests are the asker's doing, not a failure.
	if _, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "missing"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("ExportJson() of a missing locale: %v, want not found", err)
	}

	if got := sent(); len(got) != 0 {
		t.Fatalf("sent %+v for a bad request", got)
	}

	// Loading the entries fails as if the database were gone; this test's
	// connection is its own, so other tests don't notice.
	if err := db.Callback().Query().Before("gorm:query").Register("test:fail_entries", func(tx *gorm.DB) {
		if tx.Statement.Table == "entries" {
			_ = tx.AddError(stderrors.New("disk I/O error"))
		}
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := server.ExportPo(ctx, connect.NewRequest(&pb.ExportPoRequest{LocaleCode: "fx"})); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("ExportPo() error = %v, want internal", err)
	}

	if _, err := server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{LocaleIds: []string{locale.ID}})); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("ExportZip() error = %v, want internal", err)
	}

	// A download the client gave up on isn't reported.
	canceled, cancel := context.WithCancel(ctx)
	cancel()

	if _, err := server.ExportJson(canceled, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "fx"})); err == nil {
		t.Fatal("ExportJson() with a canceled context succeeded")
	}

	user := &webhook.User{ID: middleware.UserID(ctx), Name: "Test User"}
	want := []webhook.ExportFailedData{
		{Source: webhook.ExportSourceDownload, Error: "failed to query entries", Format: FormatPO, Locales: []string{"fx"}, User: user},
		{Source: webhook.ExportSourceDownload, Error: "failed to query entries", Format: "zip", LocaleIDs: []string{locale.ID}, User: user},
	}

	got := sent()
	if len(got) != len(want) {
		t.Fatalf("sent %+v\nwant %+v", got, want)
	}

	for i := range want {
		if !sameExportFailure(got[i], want[i]) {
			t.Errorf("sent %+v\nwant %+v", got[i], want[i])
		}
	}
}

func sameExportFailure(a, b webhook.ExportFailedData) bool {
	return a.Source == b.Source && a.Error == b.Error && a.Format == b.Format &&
		slices.Equal(a.Locales, b.Locales) && slices.Equal(a.LocaleIDs, b.LocaleIDs) &&
		slices.Equal(a.ScopeIDs, b.ScopeIDs) && slices.Equal(a.TagIDs, b.TagIDs) &&
		(a.User == nil) == (b.User == nil) && (a.User == nil || *a.User == *b.User)
}
