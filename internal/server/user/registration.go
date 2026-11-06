package user

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/settings"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetRegistrationSettings returns whether anyone may create an account
func (s *Server) GetRegistrationSettings(
	ctx context.Context,
	req *connect.Request[pb.GetRegistrationSettingsRequest],
) (*connect.Response[pb.GetRegistrationSettingsResponse], error) {
	result, err := s.registrationSettings(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetRegistrationSettingsResponse{Settings: result}), nil
}

// UpdateRegistrationSettings opens or closes registration
func (s *Server) UpdateRegistrationSettings(
	ctx context.Context,
	req *connect.Request[pb.UpdateRegistrationSettingsRequest],
) (*connect.Response[pb.UpdateRegistrationSettingsResponse], error) {
	if err := settings.SetBool(s.db.WithContext(ctx), models.SettingRegistrationOpen, req.Msg.Open, middleware.UserIDPtr(ctx)); err != nil {
		return nil, errors.InternalCause(err, "failed to save the registration settings")
	}

	result, err := s.registrationSettings(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.UpdateRegistrationSettingsResponse{Settings: result}), nil
}

func (s *Server) registrationSettings(ctx context.Context) (*pb.RegistrationSettings, error) {
	open, updatedAt, err := settings.Bool(s.db.WithContext(ctx), models.SettingRegistrationOpen, true)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to load the registration settings")
	}

	result := &pb.RegistrationSettings{Open: open}

	if !updatedAt.IsZero() {
		formatted := updatedAt.UTC().Format(time.RFC3339)
		result.UpdatedAt = &formatted
	}

	return result, nil
}
