package task

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListTasks returns all tasks by name, each with its newest run
func (s *Server) ListTasks(
	ctx context.Context,
	req *connect.Request[pb.ListTasksRequest],
) (*connect.Response[pb.ListTasksResponse], error) {
	var tasks []models.Task
	if err := s.db.WithContext(ctx).Order("name").Find(&tasks).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to list tasks")
	}

	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}

	runs, err := s.lastRuns(ids)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.Task, len(tasks))
	for i := range tasks {
		result[i] = converter.TaskToProto(&tasks[i], runs[tasks[i].ID])
	}

	return connect.NewResponse(&pb.ListTasksResponse{Tasks: result}), nil
}

// GetTask returns a task with its newest run
func (s *Server) GetTask(
	ctx context.Context,
	req *connect.Request[pb.GetTaskRequest],
) (*connect.Response[pb.GetTaskResponse], error) {
	task, err := s.findTask(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetTaskResponse{Task: s.taskWithLastRun(&task)}), nil
}

// CreateTask adds a task
func (s *Server) CreateTask(
	ctx context.Context,
	req *connect.Request[pb.CreateTaskRequest],
) (*connect.Response[pb.CreateTaskResponse], error) {
	task := models.Task{Type: converter.TaskTypeFromProto(req.Msg.Type)}

	if err := (settings{
		name:      req.Msg.Name,
		enabled:   req.Msg.Enabled,
		schedule:  req.Msg.Schedule,
		events:    req.Msg.Events,
		delay:     req.Msg.EventDelayMinutes,
		aiConfig:  req.Msg.AiTranslate,
		gitConfig: req.Msg.GitExport,
		now:       time.Now(),
	}).apply(s.db.WithContext(ctx), &task); err != nil {
		return nil, err
	}

	if task.Type == models.TaskTypeGitExport {
		secret, err := s.encryptToken(req.Msg.Token)
		if err != nil {
			return nil, err
		}

		task.Secret = secret
	}

	task.CreatedBy = middleware.UserIDPtr(ctx)

	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create task")
	}

	return connect.NewResponse(&pb.CreateTaskResponse{Task: converter.TaskToProto(&task, nil)}), nil
}

// UpdateTask replaces a task's settings. Disabling a task drops the runs its
// schedule and events queued; runs someone asked for stay.
func (s *Server) UpdateTask(
	ctx context.Context,
	req *connect.Request[pb.UpdateTaskRequest],
) (*connect.Response[pb.UpdateTaskResponse], error) {
	task, err := s.findTask(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if err := (settings{
		name:      req.Msg.Name,
		enabled:   req.Msg.Enabled,
		schedule:  req.Msg.Schedule,
		events:    req.Msg.Events,
		delay:     req.Msg.EventDelayMinutes,
		aiConfig:  req.Msg.AiTranslate,
		gitConfig: req.Msg.GitExport,
		now:       time.Now(),
	}).apply(s.db.WithContext(ctx), &task); err != nil {
		return nil, err
	}

	if req.Msg.Token != nil && task.Type == models.TaskTypeGitExport {
		secret, err := s.encryptToken(req.Msg.GetToken())
		if err != nil {
			return nil, err
		}

		task.Secret = secret
	}

	task.UpdatedBy = middleware.UserIDPtr(ctx)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&task).Error; err != nil {
			return err
		}

		if task.Enabled {
			return nil
		}

		return tx.Model(&models.TaskRun{}).
			Where("task_id = ? AND status = ? AND trigger <> ?", task.ID, models.RunQueued, models.TriggerManual).
			Updates(map[string]any{"status": models.RunCanceled, "finished_at": time.Now()}).Error
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to update task")
	}

	return connect.NewResponse(&pb.UpdateTaskResponse{Task: s.taskWithLastRun(&task)}), nil
}

// DeleteTask removes a task and its runs. A run in progress ends unrecorded.
func (s *Server) DeleteTask(
	ctx context.Context,
	req *connect.Request[pb.DeleteTaskRequest],
) (*connect.Response[pb.DeleteTaskResponse], error) {
	task, err := s.findTask(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.TaskRun{}).Error; err != nil {
			return err
		}

		return tx.Delete(&task).Error
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to delete task")
	}

	return connect.NewResponse(&pb.DeleteTaskResponse{}), nil
}

// taskWithLastRun converts a task with its newest run, leaving the run out if it
// can't be loaded.
func (s *Server) taskWithLastRun(task *models.Task) *pb.Task {
	runs, err := s.lastRuns([]string{task.ID})
	if err != nil {
		return converter.TaskToProto(task, nil)
	}

	return converter.TaskToProto(task, runs[task.ID])
}
