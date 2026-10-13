package export

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/gettext"
	"github.com/kazuto/parlance-server/internal/models"
)

func (s *Server) ExportPo(ctx context.Context, req *connect.Request[pb.ExportPoRequest]) (*connect.Response[pb.ExportPoResponse], error) {
	if req.Msg.LocaleCode == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale code is required"))
	}
	var locale models.Locale
	if err := s.db.WithContext(ctx).Where("code = ?", req.Msg.LocaleCode).First(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}
	query := s.db.WithContext(ctx).Model(&models.Entry{}).Preload("Localizations", "locale_id = ?", locale.ID)
	if req.Msg.ScopeId != "" {
		query = query.Joins("JOIN entry_scopes ON entry_scopes.entry_id = entries.id").Where("entry_scopes.scope_id = ?", req.Msg.ScopeId)
	}
	var entries []models.Entry
	if err := query.Find(&entries).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query entries: %w", err))
	}
	messages := make([]gettext.Entry, 0, len(entries))
	for _, entry := range entries {
		message := gettext.Entry{ID: entry.Key, Description: entry.Description}
		for _, localization := range entry.Localizations {
			if localization.LocaleID == locale.ID {
				message.Translation = localization.Translation
				break
			}
		}
		messages = append(messages, message)
	}
	filename := locale.Code + ".po"
	if req.Msg.ScopeId != "" {
		var scope models.Scope
		if err := s.db.WithContext(ctx).First(&scope, "id = ?", req.Msg.ScopeId).Error; err == nil {
			filename = fmt.Sprintf("%s_%s.po", scope.Name, locale.Code)
		}
	}
	return connect.NewResponse(&pb.ExportPoResponse{
		Format: "po", Content: gettext.Marshal(locale.Code, messages), Filename: filename,
	}), nil
}
