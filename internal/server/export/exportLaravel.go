package export

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportLaravel(
	ctx context.Context,
	req *connect.Request[pb.ExportLaravelRequest],
) (*connect.Response[pb.ExportLaravelResponse], error) {
	if req.Msg.LocaleCode == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale code is required"))
	}

	// Get locale by code
	var locale models.Locale
	if err := s.db.Where("code = ?", req.Msg.LocaleCode).First(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Build query for entries
	query := s.db.Model(&models.Entry{}).Preload("Localizations", "locale_id = ?", locale.ID)

	// Filter by scope if provided
	if req.Msg.ScopeId != "" {
		query = query.Joins("JOIN entry_scopes ON entry_scopes.entry_id = entries.id").
			Where("entry_scopes.scope_id = ?", req.Msg.ScopeId)
	}

	var entries []models.Entry
	if err := query.Find(&entries).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query entries: %w", err))
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
				content.WriteString(fmt.Sprintf("    '%s' => '%s',\n", entry.Key, translation))
				break
			}
		}
	}

	content.WriteString("];\n")

	filename := fmt.Sprintf("%s.php", req.Msg.LocaleCode)
	if req.Msg.ScopeId != "" {
		var scope models.Scope
		if err := s.db.First(&scope, "id = ?", req.Msg.ScopeId).Error; err == nil {
			filename = fmt.Sprintf("%s_%s.php", scope.Name, req.Msg.LocaleCode)
		}
	}

	return connect.NewResponse(&pb.ExportLaravelResponse{
		Format:   "laravel",
		Content:  content.String(),
		Filename: filename,
	}), nil
}
