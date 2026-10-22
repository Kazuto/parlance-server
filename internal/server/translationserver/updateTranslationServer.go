package translationserver

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateTranslationServer changes the given settings of a translation server
func (s *Server) UpdateTranslationServer(
	ctx context.Context,
	req *connect.Request[pb.UpdateTranslationServerRequest],
) (*connect.Response[pb.UpdateTranslationServerResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateTranslationServer); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation server id is required")
	}

	var server models.TranslationServer
	if err := s.db.First(&server, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrTranslationServerNotFound, "translation server not found")
	}

	if req.Msg.Name != nil {
		name := strings.TrimSpace(req.Msg.GetName())
		if name == "" {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
		}
		server.Name = name
	}

	if req.Msg.BaseUrl != nil {
		baseURL := strings.TrimSpace(req.Msg.GetBaseUrl())
		if err := validateBaseURL(baseURL); err != nil {
			return nil, err
		}
		server.BaseURL = baseURL
	}

	if req.Msg.Model != nil {
		server.Model = strings.TrimSpace(req.Msg.GetModel())
	}

	if req.Msg.ApiKey != nil {
		apiKey, err := s.encryptAPIKey(req.Msg.GetApiKey())
		if err != nil {
			return nil, err
		}
		server.APIKey = apiKey
	}

	if req.Msg.MaxConcurrency != nil {
		if err := validateMaxConcurrency(req.Msg.GetMaxConcurrency()); err != nil {
			return nil, err
		}
		server.MaxConcurrency = int(req.Msg.GetMaxConcurrency())
	}

	if req.Msg.Enabled != nil {
		server.Enabled = req.Msg.GetEnabled()
	}

	if userID, _ := ctx.Value(middleware.ContextKeyUserID).(string); userID != "" {
		server.UpdatedBy = &userID
	}

	if err := s.db.Save(&server).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update translation server")
	}

	return connect.NewResponse(&pb.UpdateTranslationServerResponse{
		TranslationServer: converter.TranslationServerToProto(&server),
	}), nil
}
