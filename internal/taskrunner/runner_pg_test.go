package taskrunner

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/forge/forgetest"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"github.com/kazuto/parlance-server/internal/translate"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// translatorFunc translates with a function.
type translatorFunc func(context.Context, translate.Request) (string, error)

func (f translatorFunc) Translate(ctx context.Context, req translate.Request) (string, error) {
	return f(ctx, req)
}

// fixedSource always returns the same translator, which may be nil.
type fixedSource struct{ translator translate.Translator }

func (s fixedSource) Translator(context.Context) (translate.Translator, error) {
	return s.translator, nil
}

func newTestRunner(t *testing.T, db *database.DB) *Runner {
	t.Helper()

	box, err := secret.New("test key")
	if err != nil {
		t.Fatalf("secret.New() error = %v", err)
	}

	r := New(db.DB, nil, box)
	r.heartbeatInterval = 10 * time.Millisecond

	return r
}

func create(t *testing.T, db *gorm.DB, value any) {
	t.Helper()

	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create %T: %v", value, err)
	}
}

func newTask(t *testing.T, db *gorm.DB, taskType string, config any) models.Task {
	t.Helper()

	task := models.Task{Name: "Task", Type: taskType}

	if config != nil {
		encoded, err := datatypesJSON(config)
		if err != nil {
			t.Fatal(err)
		}

		task.Config = encoded
	}

	create(t, db, &task)

	return task
}

func loadRun(t *testing.T, db *gorm.DB, id string) models.TaskRun {
	t.Helper()

	var run models.TaskRun
	if err := db.First(&run, "id = ?", id).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}

	return run
}

func queue(t *testing.T, db *gorm.DB, taskID string) models.TaskRun {
	t.Helper()

	run := models.TaskRun{TaskID: taskID, Trigger: models.TriggerManual, Status: models.RunQueued, RunAfter: time.Now().Add(-time.Second)}
	create(t, db, &run)

	return run
}

func TestScheduledRunsQueueOnce(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	past := time.Now().Add(-time.Minute)
	task := models.Task{Name: "Nightly", Type: models.TaskTypeGitExport, Enabled: true, Schedule: "0 3 * * *", NextRunAt: &past}
	create(t, db.DB, &task)

	runners := []*Runner{newTestRunner(t, db), newTestRunner(t, db)}

	var wg sync.WaitGroup

	for _, r := range runners {
		wg.Go(func() {
			if err := r.queueScheduled(context.Background()); err != nil {
				t.Errorf("queueScheduled() error = %v", err)
			}
		})
	}

	wg.Wait()

	var runs []models.TaskRun

	db.Where("task_id = ?", task.ID).Find(&runs)

	if len(runs) != 1 || runs[0].Trigger != models.TriggerSchedule || runs[0].Status != models.RunQueued {
		t.Fatalf("runs = %+v, want one queued scheduled run", runs)
	}

	db.First(&task, "id = ?", task.ID)

	if task.NextRunAt == nil || !task.NextRunAt.After(time.Now()) || task.NextRunAt.UTC().Hour() != 3 {
		t.Errorf("next run = %v, want the next 03:00 UTC", task.NextRunAt)
	}
}

func TestClaim(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	task := newTask(t, db.DB, models.TaskTypeGitExport, nil)
	run := queue(t, db.DB, task.ID)

	t.Run("one runner gets a run", func(t *testing.T) {
		runners := []*Runner{newTestRunner(t, db), newTestRunner(t, db), newTestRunner(t, db)}
		claimed := make([]*models.TaskRun, len(runners))

		var wg sync.WaitGroup

		for i, r := range runners {
			wg.Go(func() {
				got, err := r.claim(context.Background())
				if err != nil {
					t.Errorf("claim() error = %v", err)
				}

				claimed[i] = got
			})
		}

		wg.Wait()

		count := 0

		for _, got := range claimed {
			if got != nil {
				count++

				if got.ID != run.ID || got.Status != models.RunRunning {
					t.Errorf("claimed %+v", got)
				}
			}
		}

		if count != 1 {
			t.Errorf("%d runners claimed the run, want 1", count)
		}
	})

	t.Run("a task runs once at a time", func(t *testing.T) {
		queue(t, db.DB, task.ID)

		got, err := newTestRunner(t, db).claim(context.Background())
		if err != nil || got != nil {
			t.Errorf("claim() = %+v, %v, want nothing while the task runs", got, err)
		}
	})

	t.Run("a run waits for its time", func(t *testing.T) {
		later := newTask(t, db.DB, models.TaskTypeGitExport, nil)
		create(t, db.DB, &models.TaskRun{TaskID: later.ID, Trigger: models.TriggerEvent, Status: models.RunQueued, RunAfter: time.Now().Add(time.Hour)})

		got, err := newTestRunner(t, db).claim(context.Background())
		if err != nil || got != nil {
			t.Errorf("claim() = %+v, %v, want nothing before the run's time", got, err)
		}
	})
}

func TestPausedTasksWait(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	r := newTestRunner(t, db)
	ran := 0
	r.executors = map[string]executor{
		models.TaskTypeGitExport: func(context.Context, *models.Task, *runLog) (models.TaskRunSummary, error) {
			ran++

			return models.TaskRunSummary{}, nil
		},
	}

	past := time.Now().Add(-time.Minute)
	scheduled := models.Task{Name: "Nightly", Type: models.TaskTypeGitExport, Enabled: true, Schedule: "0 3 * * *", NextRunAt: &past}
	create(t, db.DB, &scheduled)

	manual := newTask(t, db.DB, models.TaskTypeGitExport, nil)
	run := queue(t, db.DB, manual.ID)

	if err := taskqueue.SetPaused(db.DB, true, nil); err != nil {
		t.Fatalf("SetPaused() error = %v", err)
	}

	r.Tick(context.Background())

	var queued int64

	db.Model(&models.TaskRun{}).Where("task_id = ?", scheduled.ID).Count(&queued)

	if ran != 0 || queued != 0 || loadRun(t, db.DB, run.ID).Status != models.RunQueued {
		t.Fatalf("while paused: %d ran, %d scheduled runs queued, want none", ran, queued)
	}

	if err := taskqueue.SetPaused(db.DB, false, nil); err != nil {
		t.Fatalf("SetPaused() error = %v", err)
	}

	r.Tick(context.Background())

	// The waiting run and the schedule that came due while paused both run.
	if ran != 2 || loadRun(t, db.DB, run.ID).Status != models.RunSucceeded {
		t.Errorf("after resuming: %d ran, want 2", ran)
	}
}

func TestFailStale(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	task := newTask(t, db.DB, models.TaskTypeGitExport, nil)
	old := time.Now().Add(-time.Hour)
	recent := time.Now()
	stale := models.TaskRun{TaskID: task.ID, Trigger: models.TriggerManual, Status: models.RunRunning, RunAfter: old, HeartbeatAt: &old}
	alive := models.TaskRun{TaskID: task.ID, Trigger: models.TriggerManual, Status: models.RunRunning, RunAfter: old, HeartbeatAt: &recent}

	create(t, db.DB, &stale)
	create(t, db.DB, &alive)

	if err := newTestRunner(t, db).failStale(context.Background()); err != nil {
		t.Fatalf("failStale() error = %v", err)
	}

	if got := loadRun(t, db.DB, stale.ID); got.Status != models.RunFailed || got.Error == "" || got.FinishedAt == nil {
		t.Errorf("stale run = %+v, want failed", got)
	}

	if got := loadRun(t, db.DB, alive.ID); got.Status != models.RunRunning {
		t.Errorf("live run = %+v, want running", got)
	}
}

func TestTickRecordsOutcomes(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	r := newTestRunner(t, db)
	outcome := map[string]error{}
	r.executors = map[string]executor{
		models.TaskTypeGitExport: func(_ context.Context, task *models.Task, runLog *runLog) (models.TaskRunSummary, error) {
			runLog.Printf("working on %s", task.Name)

			return models.TaskRunSummary{Outcome: "did_it"}, outcome[task.ID]
		},
	}

	good := newTask(t, db.DB, models.TaskTypeGitExport, nil)
	bad := newTask(t, db.DB, models.TaskTypeGitExport, nil)
	outcome[bad.ID] = stderrors.New("it broke")

	// More ended runs than are kept; the oldest go.
	for i := range keepRuns + 5 {
		created := time.Now().Add(-time.Duration(keepRuns+10-i) * time.Minute)
		create(t, db.DB, &models.TaskRun{TaskID: good.ID, Trigger: models.TriggerSchedule, Status: models.RunSucceeded, RunAfter: created, CreatedAt: created})
	}

	goodRun := queue(t, db.DB, good.ID)
	badRun := queue(t, db.DB, bad.ID)

	r.Tick(context.Background())

	got := loadRun(t, db.DB, goodRun.ID)
	if got.Status != models.RunSucceeded || got.RunSummary().Outcome != "did_it" || !strings.Contains(got.Log, "working on Task") || got.FinishedAt == nil {
		t.Errorf("good run = %+v", got)
	}

	got = loadRun(t, db.DB, badRun.ID)
	if got.Status != models.RunFailed || got.Error != "it broke" || !strings.Contains(got.Log, "Failed: it broke") {
		t.Errorf("bad run = %+v", got)
	}

	var count int64

	db.Model(&models.TaskRun{}).Where("task_id = ?", good.ID).Count(&count)

	if count != keepRuns {
		t.Errorf("%d runs kept, want %d", count, keepRuns)
	}

	if err := db.First(&models.TaskRun{}, "id = ?", goodRun.ID).Error; err != nil {
		t.Errorf("the newest run was dropped: %v", err)
	}
}

func TestCancelRunningRun(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	r := newTestRunner(t, db)
	started := make(chan struct{})
	r.executors = map[string]executor{
		models.TaskTypeGitExport: func(ctx context.Context, _ *models.Task, _ *runLog) (models.TaskRunSummary, error) {
			close(started)
			<-ctx.Done()

			return models.TaskRunSummary{}, ctx.Err()
		},
	}

	task := newTask(t, db.DB, models.TaskTypeGitExport, nil)
	run := queue(t, db.DB, task.ID)

	go func() {
		<-started
		db.Model(&models.TaskRun{}).Where("id = ?", run.ID).Update("cancel_requested", true)
	}()

	done := make(chan struct{})

	go func() {
		r.Tick(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}

	if got := loadRun(t, db.DB, run.ID); got.Status != models.RunCanceled || got.Error != "" {
		t.Errorf("run = %+v, want canceled", got)
	}
}

// seedLocales creates English (default), German and French.
func seedLocales(t *testing.T, db *gorm.DB) (en, de, fr models.Locale) {
	t.Helper()

	en = models.Locale{Code: "en", IsDefault: true, Names: datatypes.JSON(`{"en":"English"}`)}
	de = models.Locale{Code: "de", Names: datatypes.JSON(`{"en":"German"}`)}
	fr = models.Locale{Code: "fr", Names: datatypes.JSON(`{"en":"French"}`)}

	for _, locale := range []*models.Locale{&en, &de, &fr} {
		create(t, db, locale)
	}

	return en, de, fr
}

func seedEntry(t *testing.T, db *gorm.DB, key string, texts map[string]string, status string) models.Entry {
	t.Helper()

	entry := models.Entry{Key: key}
	create(t, db, &entry)

	for localeID, text := range texts {
		create(t, db, &models.Localization{EntryID: entry.ID, LocaleID: localeID, Translation: text, Review: models.Review{Status: status}})
	}

	return entry
}

func TestAITranslate(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	en, de, fr := seedLocales(t, db.DB)

	seedEntry(t, db.DB, "a.greeting", map[string]string{en.ID: "Hello"}, models.StatusApproved)
	seedEntry(t, db.DB, "b.farewell", map[string]string{en.ID: "Goodbye", de.ID: "Tschüss (Entwurf)"}, models.StatusDraft)
	seedEntry(t, db.DB, "c.named", map[string]string{en.ID: "Hi {name}"}, models.StatusApproved)
	seedEntry(t, db.DB, "d.untranslatable", map[string]string{de.ID: "Nur Deutsch"}, models.StatusApproved)

	// Translates by tagging the text, but drops placeholders, which a reviewer would have to fix.
	translator := translatorFunc(func(_ context.Context, req translate.Request) (string, error) {
		return fmt.Sprintf("[%s] %s", req.TargetLang, strings.ReplaceAll(req.Text, " {name}", "")), nil
	})

	run := func(t *testing.T, r *Runner, config models.AITranslateConfig) (models.TaskRunSummary, error) {
		t.Helper()

		task := newTask(t, db.DB, models.TaskTypeAITranslate, config)

		return r.aiTranslate(context.Background(), &task, &runLog{})
	}

	t.Run("fails without a translation server", func(t *testing.T) {
		if _, err := run(t, newTestRunner(t, db), models.AITranslateConfig{}); !stderrors.Is(err, errNoTranslator) {
			t.Errorf("error = %v, want errNoTranslator", err)
		}
	})

	r := newTestRunner(t, db)
	r.translators = fixedSource{translator: translator}

	t.Run("stops at the cap", func(t *testing.T) {
		summary, err := run(t, r, models.AITranslateConfig{TargetLocaleIDs: []string{fr.ID}, MaxPairs: 1})
		if err != nil {
			t.Fatalf("error = %v", err)
		}

		// a, b and c miss French; one is done.
		if summary.Translated != 1 || summary.Remaining != 2 {
			t.Errorf("summary = %+v, want 1 translated, 2 remaining", summary)
		}
	})

	t.Run("fills what is missing as drafts", func(t *testing.T) {
		summary, err := run(t, r, models.AITranslateConfig{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}

		// Missing: a/de, c/de, and in French b and c (a was done above). c loses its placeholder.
		if summary.Translated != 2 || summary.Failed != 2 || summary.Remaining != 2 {
			t.Errorf("summary = %+v, want 2 translated, 2 failed and left", summary)
		}

		var drafts []models.Localization

		db.Where("translation LIKE ?", "[%").Order("translation").Find(&drafts)

		if len(drafts) != 3 {
			t.Fatalf("drafts = %+v, want 3 machine translations", drafts)
		}

		for _, draft := range drafts {
			if draft.Status != models.StatusDraft || draft.CreatedBy != nil {
				t.Errorf("draft %q: status %q, author %v, want an authorless draft", draft.Translation, draft.Status, draft.CreatedBy)
			}
		}

		var kept models.Localization

		db.Joins("JOIN entries ON entries.id = localizations.entry_id").
			Where("entries.key = ? AND localizations.locale_id = ?", "b.farewell", de.ID).First(&kept)

		if kept.Translation != "Tschüss (Entwurf)" {
			t.Errorf("existing draft became %q", kept.Translation)
		}
	})

	t.Run("fails when nothing could be translated", func(t *testing.T) {
		summary, err := run(t, r, models.AITranslateConfig{TargetLocaleIDs: []string{de.ID, fr.ID}})
		if !stderrors.Is(err, errAllFailed) || summary.Failed != 2 {
			t.Errorf("summary = %+v, error = %v, want both left c.named translations failed", summary, err)
		}
	})
}

func TestGitExport(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	en, de, _ := seedLocales(t, db.DB)

	seedEntry(t, db.DB, "greeting", map[string]string{en.ID: "Hello"}, models.StatusApproved)
	seedEntry(t, db.DB, "farewell", map[string]string{en.ID: "Goodbye", de.ID: "Tschüss"}, models.StatusDraft)

	fake := forgetest.NewGitHub(t, map[string]string{"README.md": "app"})
	r := newTestRunner(t, db)
	r.NewHost = func(_, token string) forge.Host { return forge.NewHost(fake.Server.URL, token, nil) }

	task := newTask(t, db.DB, models.TaskTypeGitExport, models.GitExportConfig{
		Repository: fake.Repository,
		BaseBranch: "main",
		Branch:     "parlance/translations",
		Files:      []models.GitExportFile{{Format: "json", Path: "lang/{locale}.json"}},
		LocaleIDs:  []string{en.ID, de.ID},
	})

	t.Run("fails without a token", func(t *testing.T) {
		if _, err := r.gitExport(context.Background(), &task, &runLog{}); err == nil || !strings.Contains(err.Error(), "no access token") {
			t.Errorf("error = %v", err)
		}
	})

	secretToken, _ := r.box.Encrypt(fake.Token)
	task.Secret = secretToken

	t.Run("opens a pull request with approved translations", func(t *testing.T) {
		summary, err := r.gitExport(context.Background(), &task, &runLog{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}

		if summary.PullRequestURL == "" || len(summary.ChangedFiles) != 2 {
			t.Errorf("summary = %+v", summary)
		}

		files := fake.Files("parlance/translations")
		if !strings.Contains(files["lang/en.json"], `"greeting": "Hello"`) || strings.Contains(files["lang/en.json"], "farewell") {
			t.Errorf("lang/en.json = %s, want only the approved greeting", files["lang/en.json"])
		}

		if files["lang/de.json"] != "{}" {
			t.Errorf("lang/de.json = %s, want no drafts", files["lang/de.json"])
		}

		pulls := fake.Pulls()
		if len(pulls) != 1 || !strings.Contains(pulls[0].Body, "Only approved translations") {
			t.Errorf("pulls = %+v", pulls)
		}
	})

	t.Run("pushes nothing the second time", func(t *testing.T) {
		summary, err := r.gitExport(context.Background(), &task, &runLog{})
		if err != nil || summary.Outcome != forge.OutcomeUpToDate {
			t.Errorf("summary = %+v, error = %v, want up to date", summary, err)
		}
	})
}

func datatypesJSON(value any) (datatypes.JSON, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return datatypes.JSON(encoded), nil
}
