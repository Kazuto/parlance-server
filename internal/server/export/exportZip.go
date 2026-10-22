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
)

func (s *Server) ExportZip(
	ctx context.Context,
	req *connect.Request[pb.ExportZipRequest],
) (*connect.Response[pb.ExportZipResponse], error) {
	// Get all locales
	var locales []models.Locale
	if err := s.db.Find(&locales).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to query locales")
	}

	if len(locales) == 0 {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "no locales found")
	}

	// Create buffer for ZIP file
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	scopeIDs := requestedScopeIDs(req.Msg.ScopeIds)

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
		// Build query for entries
		query := s.db.Model(&models.Entry{}).Preload("Localizations", "locale_id = ?", locale.ID)
		query = filterByScopes(s.db.WithContext(ctx), query, scopeIDs)

		var entries []models.Entry
		if err := query.Find(&entries).Error; err != nil {
			return nil, errors.Internal(errors.ErrInternal, fmt.Sprintf("failed to query entries for locale %s", locale.Code))
		}

		// Build flat JSON structure
		translations := make(map[string]string)
		for _, entry := range entries {
			for _, loc := range entry.Localizations {
				if loc.LocaleID == locale.ID {
					translations[entry.Key] = loc.Translation
					break
				}
			}
		}

		// Marshal to JSON with indentation
		contentBytes, err := json.MarshalIndent(translations, "", "  ")
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

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("translations-%s.zip", timestamp)
	if len(scopeIDs) > 0 {
		// Include scope names in filename if filtering by scopes
		var names []string
		if err := s.db.Model(&models.Scope{}).Where("id IN ?", scopeIDs).Order("name").Pluck("name", &names).Error; err == nil && len(names) > 0 {
			filename = fmt.Sprintf("translations-%s-%s.zip", names[0], timestamp)
		}
	}

	return connect.NewResponse(&pb.ExportZipResponse{
		Format:   "zip",
		Content:  buf.Bytes(),
		Filename: filename,
	}), nil
}
