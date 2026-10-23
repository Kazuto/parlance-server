// Package taskrunner runs queued task runs inside the API server. Several servers
// may run it against one database: each run and each scheduled firing is claimed
// with row locks, so it happens once.
package taskrunner

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"github.com/kazuto/parlance-server/internal/translators"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Defaults of a Runner
const (
	// pollInterval is how often the runner looks for due and queued runs; a run
	// someone starts wakes it right away.
	defaultPollInterval = 15 * time.Second
	// heartbeatInterval is how often a running run shows it is alive, saves its log
	// and checks whether it was canceled.
	heartbeatInterval = 30 * time.Second
	// staleAfter is how long a run may miss heartbeats before it counts as failed,
	// e.g. because its server stopped.
	staleAfter = 2 * time.Minute
	// keepRuns is how many ended runs of each task are kept.
	keepRuns = 100
)

// Timeouts per task type
var runTimeouts = map[string]time.Duration{
	models.TaskTypeAITranslate: 30 * time.Minute,
	models.TaskTypeGitExport:   5 * time.Minute,
}

// Why a run ended early
var (
	errCanceled = stderrors.New("canceled")
	errTooLong  = stderrors.New("the run took too long")
	errShutdown = stderrors.New("the server shut down during the run")
)

// executor does the work of a task type, logging what it does.
type executor func(ctx context.Context, task *models.Task, runLog *runLog) (models.TaskRunSummary, error)

// Runner runs queued task runs one at a time.
type Runner struct {
	db          *gorm.DB
	translators translators.Source
	box         *secret.Box
	// NewHost connects to a git host; tests replace it.
	NewHost func(apiURL, token string) forge.Host

	pollInterval      time.Duration
	heartbeatInterval time.Duration
	staleAfter        time.Duration
	wake              chan struct{}
	executors         map[string]executor
}

// New creates a runner; Start runs it.
func New(db *gorm.DB, source translators.Source, box *secret.Box) *Runner {
	client := &http.Client{Timeout: time.Minute}

	r := &Runner{
		// Polling would fill the log with its queries; slow ones and errors still show.
		db:          db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Warn)}),
		translators: source,
		box:         box,
		NewHost: func(apiURL, token string) forge.Host {
			return forge.NewHost(apiURL, token, client)
		},
		pollInterval:      defaultPollInterval,
		heartbeatInterval: heartbeatInterval,
		staleAfter:        staleAfter,
		wake:              make(chan struct{}, 1),
	}
	r.executors = map[string]executor{
		models.TaskTypeAITranslate: r.aiTranslate,
		models.TaskTypeGitExport:   r.gitExport,
	}

	return r
}

// Wake makes the runner look for work now instead of at its next poll, e.g.
// after a run was queued to start right away. It is safe on a nil runner.
func (r *Runner) Wake() {
	if r == nil {
		return
	}

	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// Start runs the runner until ctx ends. The returned channel closes once the run
// in progress, if any, has been ended.
func (r *Runner) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(r.pollInterval)
		defer ticker.Stop()

		for {
			r.Tick(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-r.wake:
			}
		}
	}()

	return done
}

// Tick queues the scheduled runs that are due, fails runs that stopped beating,
// and runs every queued run that may start. While tasks are paused, it only fails
// stale runs; schedules that came due fire once they are resumed.
func (r *Runner) Tick(ctx context.Context) {
	if err := r.failStale(ctx); err != nil {
		log.Printf("tasks: fail stale runs: %v", err)
	}

	paused, _, err := taskqueue.Paused(r.db.WithContext(ctx))
	if err != nil {
		log.Printf("tasks: read whether tasks are paused: %v", err)

		return
	}

	if paused {
		return
	}

	if err := r.queueScheduled(ctx); err != nil {
		log.Printf("tasks: queue scheduled runs: %v", err)
	}

	for ctx.Err() == nil {
		run, err := r.claim(ctx)
		if err != nil {
			log.Printf("tasks: claim a run: %v", err)

			return
		}

		if run == nil {
			return
		}

		r.execute(ctx, run)
	}
}

// queueScheduled queues a run of each enabled task whose schedule is due, and moves
// its next run on. Tasks another server is handling are skipped.
func (r *Runner) queueScheduled(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		var tasks []models.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND next_run_at <= ?", true, now).
			Find(&tasks).Error; err != nil {
			return err
		}

		for i := range tasks {
			task := &tasks[i]

			if _, err := taskqueue.Enqueue(tx, task.ID, models.TriggerSchedule, nil, now); err != nil {
				return err
			}

			next, err := taskqueue.NextRun(task, now)
			if err != nil {
				// Saved schedules were checked, so this means a bad one got in;
				// stop firing it rather than every poll.
				log.Printf("tasks: task %s has an invalid schedule: %v", task.ID, err)
			}

			if err := tx.Model(task).Update("next_run_at", next).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// failStale ends runs whose server stopped beating for them as failed.
func (r *Runner) failStale(ctx context.Context) error {
	now := time.Now()

	return r.db.WithContext(ctx).Model(&models.TaskRun{}).
		Where("status = ? AND heartbeat_at < ?", models.RunRunning, now.Add(-r.staleAfter)).
		Updates(map[string]any{
			"status":      models.RunFailed,
			"error":       "the run stopped responding, e.g. because its server stopped",
			"finished_at": now,
		}).Error
}

// claim marks the next queued run that may start as running and returns it, or nil
// when there is none. Runs of a task that is running already wait.
func (r *Runner) claim(ctx context.Context) (*models.TaskRun, error) {
	var claimed *models.TaskRun

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		var runs []models.TaskRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND run_after <= ?", models.RunQueued, now).
			Where("NOT EXISTS (SELECT 1 FROM task_runs running WHERE running.task_id = task_runs.task_id AND running.status = ?)", models.RunRunning).
			Order("run_after").
			Limit(1).
			Find(&runs).Error; err != nil {
			return err
		}

		if len(runs) == 0 {
			return nil
		}

		run := &runs[0]
		run.Status = models.RunRunning
		run.StartedAt = &now
		run.HeartbeatAt = &now

		if err := tx.Model(run).Select("status", "started_at", "heartbeat_at").Updates(run).Error; err != nil {
			return err
		}

		claimed = run

		return nil
	})

	return claimed, err
}

// execute runs a claimed run to its end and records how it went.
func (r *Runner) execute(ctx context.Context, run *models.TaskRun) {
	runLog := &runLog{}

	var task models.Task
	if err := r.db.WithContext(ctx).First(&task, "id = ?", run.TaskID).Error; err != nil {
		r.finish(run, runLog, models.TaskRunSummary{}, fmt.Errorf("load the task: %w", err))

		return
	}

	work, ok := r.executors[task.Type]
	if !ok {
		r.finish(run, runLog, models.TaskRunSummary{}, fmt.Errorf("unknown task type %q", task.Type))

		return
	}

	runCtx, cancelTimeout := context.WithTimeoutCause(ctx, runTimeouts[task.Type], errTooLong)
	defer cancelTimeout()

	runCtx, cancelRun := context.WithCancelCause(runCtx)
	defer cancelRun(nil)

	stopBeating := r.heartbeat(run, runLog, func() { cancelRun(errCanceled) })

	runLog.Printf("Started (%s)", run.Trigger)

	summary, err := work(runCtx, &task, runLog)

	stopBeating()

	// Work cut short reports why it was, rather than how that showed.
	if runCtx.Err() != nil {
		err = context.Cause(runCtx)
		if ctx.Err() != nil {
			err = errShutdown
		}
	}

	r.finish(run, runLog, summary, err)
}

// heartbeat keeps the run marked alive and saves its log until stopped, and calls
// cancel once someone asks to cancel the run.
func (r *Runner) heartbeat(run *models.TaskRun, runLog *runLog, cancel func()) (stop func()) {
	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Go(func() {
		ticker := time.NewTicker(r.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
			}

			updates := map[string]any{"heartbeat_at": time.Now()}
			if text, changed := runLog.takeChanged(); changed {
				updates["log"] = text
			}

			if err := r.db.Model(&models.TaskRun{}).Where("id = ?", run.ID).Updates(updates).Error; err != nil {
				log.Printf("tasks: heartbeat of run %s: %v", run.ID, err)
			}

			var current models.TaskRun
			if err := r.db.Select("cancel_requested").First(&current, "id = ?", run.ID).Error; err == nil && current.CancelRequested {
				cancel()
			}
		}
	})

	return func() {
		close(done)
		wg.Wait()
	}
}

// finish records how the run ended, unless it was ended already, e.g. as stale,
// and drops the task's oldest ended runs.
func (r *Runner) finish(run *models.TaskRun, runLog *runLog, summary models.TaskRunSummary, err error) {
	status := models.RunSucceeded
	message := ""

	switch {
	case stderrors.Is(err, errCanceled):
		status = models.RunCanceled

		runLog.Printf("Canceled")
	case err != nil:
		status = models.RunFailed
		message = err.Error()

		runLog.Printf("Failed: %s", message)
	default:
		runLog.Printf("Done")
	}

	encoded, encodeErr := json.Marshal(summary)
	if encodeErr != nil {
		encoded = []byte("{}")
	}

	// The run's context may have ended, so this uses its own.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := r.db.WithContext(ctx)

	if err := db.Model(&models.TaskRun{}).
		Where("id = ? AND status = ?", run.ID, models.RunRunning).
		Updates(map[string]any{
			"status":      status,
			"error":       message,
			"summary":     datatypes.JSON(encoded),
			"log":         runLog.String(),
			"finished_at": time.Now(),
		}).Error; err != nil {
		log.Printf("tasks: finish run %s: %v", run.ID, err)
	}

	kept := db.Model(&models.TaskRun{}).Select("id").
		Where("task_id = ?", run.TaskID).
		Order("created_at DESC").
		Limit(keepRuns)
	if err := db.
		Where("task_id = ? AND status NOT IN ? AND id NOT IN (?)", run.TaskID, []string{models.RunQueued, models.RunRunning}, kept).
		Delete(&models.TaskRun{}).Error; err != nil {
		log.Printf("tasks: drop old runs of task %s: %v", run.TaskID, err)
	}
}
