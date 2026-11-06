package task

import (
	"context"
	stderrors "errors"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/taskqueue"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// RunTask queues a run that starts right away; a run queued already is moved up.
// Disabled tasks run too, as only their schedule and events are off.
func (s *Server) RunTask(
	ctx context.Context,
	req *connect.Request[pb.RunTaskRequest],
) (*connect.Response[pb.RunTaskResponse], error) {
	task, err := s.findTask(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	run, err := taskqueue.Enqueue(s.db.DB.WithContext(ctx), task.ID, models.TriggerManual, middleware.UserIDPtr(ctx), time.Now())
	if err != nil {
		return nil, errors.InternalCause(err, "failed to queue the run")
	}

	if s.wake != nil {
		s.wake()
	}

	return connect.NewResponse(&pb.RunTaskResponse{Run: converter.TaskRunToProto(&run, false)}), nil
}

// ListTaskRuns returns a task's runs, newest first, without their logs
func (s *Server) ListTaskRuns(
	ctx context.Context,
	req *connect.Request[pb.ListTaskRunsRequest],
) (*connect.Response[pb.ListTaskRunsResponse], error) {
	if _, err := s.findTask(req.Msg.TaskId); err != nil {
		return nil, err
	}

	var runs []models.TaskRun

	query := s.db.WithContext(ctx).Model(&models.TaskRun{}).Omit("log").
		Where("task_id = ?", req.Msg.TaskId).
		Order("created_at DESC")

	pagination, err := common.Paginate(query, req.Msg.Pagination, &runs, "TriggeredByUser")
	if err != nil {
		return nil, err
	}

	result := make([]*pb.TaskRun, len(runs))
	for i := range runs {
		result[i] = converter.TaskRunToProto(&runs[i], false)
	}

	return connect.NewResponse(&pb.ListTaskRunsResponse{Runs: result, Pagination: pagination}), nil
}

// GetTaskRun returns a run with its log
func (s *Server) GetTaskRun(
	ctx context.Context,
	req *connect.Request[pb.GetTaskRunRequest],
) (*connect.Response[pb.GetTaskRunResponse], error) {
	run, err := s.findRun(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetTaskRunResponse{Run: converter.TaskRunToProto(&run, true)}), nil
}

// CancelTaskRun drops a queued run, or asks a running one to stop
func (s *Server) CancelTaskRun(
	ctx context.Context,
	req *connect.Request[pb.CancelTaskRunRequest],
) (*connect.Response[pb.CancelTaskRunResponse], error) {
	run, err := s.findRun(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if err := taskqueue.Cancel(s.db.DB.WithContext(ctx), run.ID); err != nil {
		if stderrors.Is(err, taskqueue.ErrNotCancelable) {
			return nil, errors.FailedPrecondition(errors.ErrFailedPrecondition, "the run has ended")
		}

		return nil, errors.InternalCause(err, "failed to cancel the run")
	}

	run, err = s.findRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.CancelTaskRunResponse{Run: converter.TaskRunToProto(&run, true)}), nil
}

func (s *Server) findRun(ctx context.Context, id string) (models.TaskRun, error) {
	if id == "" {
		return models.TaskRun{}, errors.InvalidInput(errors.ErrInvalidInput, "run id is required")
	}

	return common.FindByID[models.TaskRun](s.db.WithContext(ctx).Preload("TriggeredByUser"), id, errors.ErrTaskRunNotFound, "task run not found")
}
