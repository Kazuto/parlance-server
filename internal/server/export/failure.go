package export

import (
	"context"
	"log"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/webhook"
)

// reportFailure sends a download that failed to the webhooks subscribed to
// export.failed. Only the server's own errors count, not bad requests or a client
// that went away.
func (s *Server) reportFailure(ctx context.Context, err error, data webhook.ExportFailedData) {
	if code := connect.CodeOf(err); ctx.Err() != nil || (code != connect.CodeInternal && code != connect.CodeUnknown) {
		return
	}

	data.Error = errors.MessageOf(err)
	db := s.db.WithContext(context.WithoutCancel(ctx))

	if err := webhook.Emit(db, models.WebhookEventExportFailed, func() (any, error) {
		return webhook.FailedDownload(db, middleware.UserIDPtr(ctx), data)
	}); err != nil {
		log.Printf("exports: report a failed download: %v", err)
	}
}
