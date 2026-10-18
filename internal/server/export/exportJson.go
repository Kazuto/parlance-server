package export

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportJson(
	ctx context.Context,
	req *connect.Request[pb.ExportJsonRequest],
) (*connect.Response[pb.ExportJsonResponse], error) {
	if req.Msg.LocaleCode == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale code is required")
	}

	// Get locale by code
	var locale models.Locale
	if err := s.db.Where("code = ?", req.Msg.LocaleCode).First(&locale).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	// Build query for entries
	query := s.db.Model(&models.Entry{}).Preload("Localizations", "locale_id = ?", locale.ID)

	scopeIDs := requestedScopeIDs(req.Msg.ScopeIds)
	query = filterByScopes(s.db.WithContext(ctx), query, scopeIDs)

	var entries []models.Entry
	if err := query.Find(&entries).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to query entries")
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
		return nil, errors.Internal(errors.ErrInternal, "failed to marshal JSON")
	}

	filename := exportFilename(s.db.WithContext(ctx), scopeIDs, req.Msg.LocaleCode, "json")

	return connect.NewResponse(&pb.ExportJsonResponse{
		Format:   "json",
		Content:  string(contentBytes),
		Filename: filename,
	}), nil
}
