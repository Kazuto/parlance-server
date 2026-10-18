package export

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportPhp(
	ctx context.Context,
	req *connect.Request[pb.ExportPhpRequest],
) (*connect.Response[pb.ExportPhpResponse], error) {
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

	// Build Laravel PHP array format
	var content strings.Builder
	content.WriteString("<?php\n\n")
	content.WriteString("return [\n")

	for _, entry := range entries {
		for _, loc := range entry.Localizations {
			if loc.LocaleID == locale.ID {
				// Escape single quotes in translation
				translation := strings.ReplaceAll(loc.Translation, "'", "\\'")
				fmt.Fprintf(&content, "    '%s' => '%s',\n", entry.Key, translation)
				break
			}
		}
	}

	content.WriteString("];\n")

	filename := exportFilename(s.db.WithContext(ctx), scopeIDs, req.Msg.LocaleCode, "php")

	return connect.NewResponse(&pb.ExportPhpResponse{
		Format:   "php",
		Content:  content.String(),
		Filename: filename,
	}), nil
}
