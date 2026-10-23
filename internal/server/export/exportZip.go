package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
)

func (s *Server) ExportZip(
	ctx context.Context,
	req *connect.Request[pb.ExportZipRequest],
) (*connect.Response[pb.ExportZipResponse], error) {
	db := s.db.WithContext(ctx)

	var locales []models.Locale

	if err := db.Order("code").Find(&locales).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to query locales")
	}

	if len(locales) == 0 {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "no locales found")
	}

	// Create buffer for ZIP file
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	scopeIDs := common.UniqueIDs(req.Msg.ScopeIds)

	// Create manifest
	manifest := map[string]interface{}{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"format":      "json",
		"version":     "1.0",
	}
	localeCodes := make([]string, 0, len(locales))

	if len(scopeIDs) > 0 {
		manifest["scope_ids"] = scopeIDs
	}

	// Export each locale as a JSON file
	for _, locale := range locales {
		entries, err := loadEntries(db, locale.ID, scopeIDs)
		if err != nil {
			return nil, err
		}

		contentBytes, err := json.MarshalIndent(translationMap(entries), "", "  ")
		if err != nil {
			return nil, errors.Internal(errors.ErrInternal, fmt.Sprintf("failed to marshal JSON for locale %s", locale.Code))
		}

		// Add JSON file to ZIP
		filename := locale.Code + ".json"
		writer, err := zipWriter.Create(filename)

		if err != nil {
			return nil, errors.Internal(errors.ErrInternal, fmt.Sprintf("failed to create ZIP entry for %s", filename))
		}

		if _, err := writer.Write(contentBytes); err != nil {
			return nil, errors.Internal(errors.ErrInternal, fmt.Sprintf("failed to write ZIP entry for %s", filename))
		}

		localeCodes = append(localeCodes, locale.Code)
	}

	manifest["locales"] = localeCodes

	// Add manifest.json to ZIP
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to marshal manifest")
	}

	manifestWriter, err := zipWriter.Create("manifest.json")
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create manifest entry")
	}

	if _, err := manifestWriter.Write(manifestBytes); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to write manifest entry")
	}

	// Close ZIP writer
	if err := zipWriter.Close(); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to close ZIP archive")
	}

	// e.g. "translations-backend-shared-2006-01-02.zip"
	filename := "translations-"
	if names := scopeNames(db, scopeIDs); names != "" {
		filename += names + "-"
	}

	filename += time.Now().Format("2006-01-02") + ".zip"

	return connect.NewResponse(&pb.ExportZipResponse{
		Format:   "zip",
		Content:  buf.Bytes(),
		Filename: filename,
	}), nil
}
