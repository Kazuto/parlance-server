package task

import (
	"net/http"
	"time"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
)

// Server implements the TaskService
type Server struct {
	db  *database.DB
	box *secret.Box
	// newHost connects to a git host for CheckTaskAccess.
	newHost func(apiURL, token string) forge.Host
	// wake tells the runner a run may start now; nil without a runner here.
	wake func()
}

// NewServer creates a new TaskService server. wake, if given, is called when a run
// is queued to start right away.
func NewServer(db *database.DB, box *secret.Box, wake func()) parlancev1connect.TaskServiceHandler {
	client := &http.Client{Timeout: 30 * time.Second}

	return &Server{
		db:  db,
		box: box,
		newHost: func(apiURL, token string) forge.Host {
			return forge.NewHost(apiURL, token, client)
		},
		wake: wake,
	}
}

func (s *Server) findTask(id string) (models.Task, error) {
	var task models.Task
	if id == "" {
		return task, errors.InvalidInput(errors.ErrInvalidInput, "task id is required")
	}

	result := s.db.Where("id = ?", id).Limit(1).Find(&task)
	if result.Error != nil {
		return task, errors.InternalCause(result.Error, "failed to load task")
	}

	if result.RowsAffected == 0 {
		return task, errors.NotFound(errors.ErrTaskNotFound, "task not found")
	}

	return task, nil
}

// lastRuns returns the newest run of each of the tasks, by task id.
func (s *Server) lastRuns(taskIDs []string) (map[string]*models.TaskRun, error) {
	runs := map[string]*models.TaskRun{}
	if len(taskIDs) == 0 {
		return runs, nil
	}

	var rows []models.TaskRun
	if err := s.db.Omit("log").Preload("TriggeredByUser").
		Where("task_id IN ?", taskIDs).
		Where("created_at = (SELECT MAX(newest.created_at) FROM task_runs newest WHERE newest.task_id = task_runs.task_id)").
		Find(&rows).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load task runs")
	}

	for i := range rows {
		runs[rows[i].TaskID] = &rows[i]
	}

	return runs, nil
}

// encryptToken returns the token to store: encrypted, or empty for no token.
func (s *Server) encryptToken(token string) (string, error) {
	if token == "" {
		return "", nil
	}

	encrypted, err := s.box.Encrypt(token)
	if err != nil {
		return "", errors.FailedPrecondition(errors.ErrEncryptionKeyMissing, "set ENCRYPTION_KEY on the server to store access tokens")
	}

	return encrypted, nil
}
