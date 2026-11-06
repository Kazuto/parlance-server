package taskqueue_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"gorm.io/gorm"
)

func TestNextRun(t *testing.T) {
	after := time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)

	next, err := taskqueue.NextRun(&models.Task{Enabled: true, Schedule: "0 3 * * *"}, after)
	if err != nil || next == nil || !next.Equal(time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("daily at 03:00: next = %v, err = %v", next, err)
	}

	// Schedules run in UTC, whatever zone the time is given in.
	berlin := time.FixedZone("CEST", 2*60*60)

	next, err = taskqueue.NextRun(&models.Task{Enabled: true, Schedule: "@hourly"}, after.In(berlin))
	if err != nil || next == nil || !next.Equal(time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)) {
		t.Errorf("hourly: next = %v, err = %v", next, err)
	}

	for _, task := range []models.Task{{Enabled: false, Schedule: "@hourly"}, {Enabled: true}} {
		if next, err := taskqueue.NextRun(&task, after); next != nil || err != nil {
			t.Errorf("%+v: next = %v, err = %v, want none", task, next, err)
		}
	}

	if _, err := taskqueue.NextRun(&models.Task{Enabled: true, Schedule: "every day"}, after); err == nil {
		t.Error("an invalid schedule parsed")
	}
}

// forEachDB runs the test on SQLite and, when configured, on Postgres.
func forEachDB(t *testing.T, test func(t *testing.T, db *gorm.DB)) {
	t.Run("sqlite", func(t *testing.T) {
		_, database := testutil.Setup(t)
		test(t, database.DB)
	})
	t.Run("postgres", func(t *testing.T) {
		test(t, testutil.SetupPostgresDB(t).DB)
	})
}

func createTask(t *testing.T, db *gorm.DB, task models.Task) models.Task {
	t.Helper()

	if task.Name == "" {
		task.Name = "Export"
	}

	task.Type = models.TaskTypeGitExport
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	return task
}

func queuedRuns(t *testing.T, db *gorm.DB, taskID string) []models.TaskRun {
	t.Helper()

	var runs []models.TaskRun
	if err := db.Where("task_id = ? AND status = ?", taskID, models.RunQueued).Find(&runs).Error; err != nil {
		t.Fatalf("load runs: %v", err)
	}

	return runs
}

func TestEnqueueKeepsOneQueuedRun(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		task := createTask(t, db, models.Task{Enabled: true})
		now := time.Now()

		first, err := taskqueue.Enqueue(db, task.ID, models.TriggerEvent, nil, now.Add(10*time.Minute))
		if err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}

		// A later trigger joins the queued run without delaying it.
		second, err := taskqueue.Enqueue(db, task.ID, models.TriggerSchedule, nil, now.Add(20*time.Minute))
		if err != nil || second.ID != first.ID || second.Trigger != models.TriggerEvent {
			t.Fatalf("second = %+v, err = %v, want the first run unchanged", second, err)
		}

		// Asking for a run now moves it up.
		starter := models.User{Email: "starter@example.com", Name: "Starter", PasswordHash: "unused"}
		if err := db.Create(&starter).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}

		user := starter.ID

		manual, err := taskqueue.Enqueue(db, task.ID, models.TriggerManual, &user, now)
		if err != nil || manual.ID != first.ID || manual.Trigger != models.TriggerManual || manual.RunAfter.After(now.Add(time.Second)) {
			t.Fatalf("manual = %+v, err = %v, want the first run moved up", manual, err)
		}

		if runs := queuedRuns(t, db, task.ID); len(runs) != 1 || runs[0].TriggeredBy == nil || *runs[0].TriggeredBy != user {
			t.Errorf("queued runs = %+v, want one triggered by the user", runs)
		}
	})
}

func TestEmitQueuesSubscribedTasks(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		subscribed := createTask(t, db, models.Task{Enabled: true, Events: []string{models.EventTranslationsApproved}, EventDelayMinutes: 5})
		disabled := createTask(t, db, models.Task{Enabled: false, Events: []string{models.EventTranslationsApproved}})
		other := createTask(t, db, models.Task{Enabled: true})

		before := time.Now()

		for range 3 {
			if err := taskqueue.Emit(db, models.EventTranslationsApproved); err != nil {
				t.Fatalf("Emit() error = %v", err)
			}
		}

		runs := queuedRuns(t, db, subscribed.ID)
		if len(runs) != 1 || runs[0].Trigger != models.TriggerEvent {
			t.Fatalf("subscribed task runs = %+v, want one event run", runs)
		}

		if delay := runs[0].RunAfter.Sub(before); delay < 5*time.Minute || delay > 6*time.Minute {
			t.Errorf("run starts %v after the event, want its 5 minute delay", delay)
		}

		if len(queuedRuns(t, db, disabled.ID)) != 0 || len(queuedRuns(t, db, other.ID)) != 0 {
			t.Error("a disabled or unsubscribed task was queued")
		}
	})
}

func TestCancel(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		task := createTask(t, db, models.Task{})

		queued, err := taskqueue.Enqueue(db, task.ID, models.TriggerManual, nil, time.Now())
		if err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}

		if err := taskqueue.Cancel(db, queued.ID); err != nil {
			t.Fatalf("Cancel(queued) error = %v", err)
		}

		db.First(&queued, "id = ?", queued.ID)

		if queued.Status != models.RunCanceled || queued.FinishedAt == nil {
			t.Errorf("queued run after cancel = %+v", queued)
		}

		if err := taskqueue.Cancel(db, queued.ID); !errors.Is(err, taskqueue.ErrNotCancelable) {
			t.Errorf("Cancel(canceled) error = %v, want ErrNotCancelable", err)
		}

		running := models.TaskRun{TaskID: task.ID, Trigger: models.TriggerManual, Status: models.RunRunning, RunAfter: time.Now()}
		db.Create(&running)

		if err := taskqueue.Cancel(db, running.ID); err != nil {
			t.Fatalf("Cancel(running) error = %v", err)
		}

		db.First(&running, "id = ?", running.ID)

		if running.Status != models.RunRunning || !running.CancelRequested {
			t.Errorf("running run after cancel = %+v, want it asked to stop", running)
		}
	})
}
