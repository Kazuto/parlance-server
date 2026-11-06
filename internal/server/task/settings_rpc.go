package task

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/taskqueue"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetTaskSettings returns the settings of all tasks
func (s *Server) GetTaskSettings(
	ctx context.Context,
	req *connect.Request[pb.GetTaskSettingsRequest],
) (*connect.Response[pb.GetTaskSettingsResponse], error) {
	settings, err := s.taskSettings(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetTaskSettingsResponse{Settings: settings}), nil
}

// UpdateTaskSettings pauses or resumes all tasks. Resuming wakes the runner, so
// waiting runs start right away.
func (s *Server) UpdateTaskSettings(
	ctx context.Context,
	req *connect.Request[pb.UpdateTaskSettingsRequest],
) (*connect.Response[pb.UpdateTaskSettingsResponse], error) {
	if err := taskqueue.SetPaused(s.db.DB.WithContext(ctx), req.Msg.Paused, middleware.UserIDPtr(ctx)); err != nil {
		return nil, errors.InternalCause(err, "failed to save the task settings")
	}

	if !req.Msg.Paused && s.wake != nil {
		s.wake()
	}

	settings, err := s.taskSettings(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.UpdateTaskSettingsResponse{Settings: settings}), nil
}

func (s *Server) taskSettings(ctx context.Context) (*pb.TaskSettings, error) {
	paused, updatedAt, err := taskqueue.Paused(s.db.DB.WithContext(ctx))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to load the task settings")
	}

	settings := &pb.TaskSettings{Paused: paused}

	if !updatedAt.IsZero() {
		formatted := updatedAt.UTC().Format(time.RFC3339)
		settings.UpdatedAt = &formatted
	}

	return settings, nil
}
