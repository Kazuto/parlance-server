// Package taskqueue queues task runs: by schedule, after events, or on request.
// Package taskrunner runs them.
package taskqueue

import (
	stderrors "errors"
	"slices"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/settings"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ParseSchedule parses a standard 5-field cron expression, such as "0 3 * * *",
// or a descriptor such as "@daily". Schedules run in UTC.
func ParseSchedule(schedule string) (cron.Schedule, error) {
	return cron.ParseStandard(schedule)
}

// NextRun returns when the task's schedule fires next after the time, or nil while
// the task is disabled or has no schedule.
func NextRun(task *models.Task, after time.Time) (*time.Time, error) {
	if !task.Enabled || task.Schedule == "" {
		return nil, nil
	}

	schedule, err := ParseSchedule(task.Schedule)
	if err != nil {
		return nil, err
	}

	next := schedule.Next(after.UTC())

	return &next, nil
}

// Enqueue queues a run of the task that starts at runAfter. A task has at most one
// queued run, so when one is queued already, the trigger joins it instead: it moves
// up to runAfter if that is sooner, as when someone asks for a run now.
func Enqueue(tx *gorm.DB, taskID, trigger string, triggeredBy *string, runAfter time.Time) (models.TaskRun, error) {
	run := models.TaskRun{
		TaskID:      taskID,
		Trigger:     trigger,
		TriggeredBy: triggeredBy,
		Status:      models.RunQueued,
		RunAfter:    runAfter.UTC(),
	}

	// The conflict target matches the partial unique index on queued runs.
	result := tx.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "task_id"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "status = 'queued'"}}},
		DoNothing:   true,
	}).Create(&run)
	if result.Error != nil {
		return run, result.Error
	}

	if result.RowsAffected > 0 {
		return run, nil
	}

	var queued models.TaskRun
	if err := tx.Where("task_id = ? AND status = ?", taskID, models.RunQueued).First(&queued).Error; err != nil {
		return queued, err
	}

	if runAfter.Before(queued.RunAfter) {
		queued.RunAfter = runAfter.UTC()
		queued.Trigger = trigger
		queued.TriggeredBy = triggeredBy

		if err := tx.Model(&queued).Select("run_after", "trigger", "triggered_by").Updates(&queued).Error; err != nil {
			return queued, err
		}
	}

	return queued, nil
}

// Emit queues a run of each enabled task that runs after the event, its delay after
// now. Further events before that run starts join it, so a burst of them runs once.
// It runs in the transaction of the change, so a change that is rolled back emits nothing.
func Emit(tx *gorm.DB, event string) error {
	var tasks []models.Task
	if err := tx.Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return err
	}

	now := time.Now()

	for _, task := range tasks {
		if !slices.Contains(task.Events, event) {
			continue
		}

		delay := time.Duration(task.EventDelayMinutes) * time.Minute

		if _, err := Enqueue(tx, task.ID, models.TriggerEvent, nil, now.Add(delay)); err != nil {
			return err
		}
	}

	return nil
}

// Paused tells whether tasks are paused, an admin's setting. Nothing is scheduled
// or run while they are; triggers still queue runs, which start once resumed.
func Paused(db *gorm.DB) (bool, time.Time, error) {
	return settings.Bool(db, models.SettingTasksPaused, false)
}

// SetPaused pauses or resumes all tasks.
func SetPaused(db *gorm.DB, paused bool, by *string) error {
	return settings.SetBool(db, models.SettingTasksPaused, paused, by)
}

// ErrNotCancelable is returned for runs that have ended.
var ErrNotCancelable = stderrors.New("the run has ended")

// Cancel drops a queued run, or asks a running one to stop; the runner then ends it.
// The conditions on the status keep it right while the runner claims the run.
func Cancel(tx *gorm.DB, runID string) error {
	result := tx.Model(&models.TaskRun{}).
		Where("id = ? AND status = ?", runID, models.RunQueued).
		Updates(map[string]any{"status": models.RunCanceled, "finished_at": time.Now()})
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}

	result = tx.Model(&models.TaskRun{}).
		Where("id = ? AND status = ?", runID, models.RunRunning).
		Update("cancel_requested", true)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}

	return ErrNotCancelable
}
