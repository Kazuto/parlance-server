package task

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/forge/forgetest"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func setup(t *testing.T) (context.Context, *Server, *forgetest.GitHub, *int) {
	t.Helper()

	ctx, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM task_runs`)
		db.Exec(`DELETE FROM tasks`)
		db.Exec(`DELETE FROM locales`)
	})

	box, _ := secret.New("test-secret")
	fake := forgetest.NewGitHub(t, map[string]string{})
	wakes := 0

	server := &Server{
		db:  db,
		box: box,
		newHost: func(_, token string) forge.Host {
			return forge.NewHost(fake.Server.URL, token, nil)
		},
		wake: func() { wakes++ },
	}

	return ctx, server, fake, &wakes
}

func gitExport(repository string) *pb.GitExportConfig {
	return &pb.GitExportConfig{
		Repository: repository,
		BaseBranch: "main",
		Branch:     "parlance/translations",
		Files:      []*pb.GitExportFile{{Format: pb.ExportFormat_EXPORT_FORMAT_JSON, Path: "lang/{locale}.json"}},
	}
}

func TestCreateTask(t *testing.T) {
	ctx, server, _, _ := setup(t)

	locale := models.Locale{Code: "de", Names: datatypes.JSON(`{}`)}
	server.db.Create(&locale)

	t.Run("creates a scheduled machine translation", func(t *testing.T) {
		res, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
			Name:              " Nightly ",
			Type:              pb.TaskType_TASK_TYPE_AI_TRANSLATE,
			Enabled:           true,
			Schedule:          "0 3 * * *",
			Events:            []pb.TaskEvent{pb.TaskEvent_TASK_EVENT_TRANSLATIONS_APPROVED, pb.TaskEvent_TASK_EVENT_TRANSLATIONS_APPROVED},
			EventDelayMinutes: 15,
			AiTranslate:       &pb.AiTranslateConfig{TargetLocaleIds: []string{locale.ID}, ApplyTerminology: true},
		}))
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		task := res.Msg.Task
		if task.Name != "Nightly" || task.NextRunAt == nil || len(task.Events) != 1 || task.EventDelayMinutes != 15 {
			t.Errorf("task = %+v", task)
		}

		if task.AiTranslate == nil || task.AiTranslate.TargetLocaleIds[0] != locale.ID || !task.AiTranslate.ApplyTerminology || task.GitExport != nil {
			t.Errorf("config = %+v", task.AiTranslate)
		}
	})

	t.Run("keeps a zero event delay", func(t *testing.T) {
		res, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
			Name: "Right away", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE, Enabled: true,
			Events: []pb.TaskEvent{pb.TaskEvent_TASK_EVENT_TRANSLATIONS_APPROVED},
		}))
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		var stored models.Task

		server.db.First(&stored, "id = ?", res.Msg.Task.Id)

		if stored.EventDelayMinutes != 0 {
			t.Errorf("stored delay = %d, want 0", stored.EventDelayMinutes)
		}
	})

	t.Run("keeps the token encrypted and to itself", func(t *testing.T) {
		res, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
			Name: "Ship", Type: pb.TaskType_TASK_TYPE_GIT_EXPORT, GitExport: gitExport("acme/app"), Token: "ghp_secret",
		}))
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		if !res.Msg.Task.HasToken || res.Msg.Task.NextRunAt != nil {
			t.Errorf("task = %+v, want a token and no schedule", res.Msg.Task)
		}

		var stored models.Task

		server.db.First(&stored, "id = ?", res.Msg.Task.Id)

		if plain, err := server.box.Decrypt(stored.Secret); err != nil || plain != "ghp_secret" {
			t.Errorf("stored token = %q, %v", plain, err)
		}
	})

	t.Run("rejects invalid settings", func(t *testing.T) {
		withGit := func(change func(*pb.GitExportConfig)) *pb.CreateTaskRequest {
			config := gitExport("acme/app")
			change(config)

			return &pb.CreateTaskRequest{Name: "Ship", Type: pb.TaskType_TASK_TYPE_GIT_EXPORT, GitExport: config}
		}

		testutil.ExpectInvalidArgument(t, ctx, server.CreateTask,
			&pb.CreateTaskRequest{Name: "No type"},
			&pb.CreateTaskRequest{Name: " ", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE},
			&pb.CreateTaskRequest{Name: "Bad cron", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE, Schedule: "every night"},
			&pb.CreateTaskRequest{Name: "Bad delay", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE, EventDelayMinutes: -1},
			&pb.CreateTaskRequest{Name: "Bad event", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE, Events: []pb.TaskEvent{pb.TaskEvent_TASK_EVENT_UNSPECIFIED}},
			&pb.CreateTaskRequest{Name: "Unknown locale", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE, AiTranslate: &pb.AiTranslateConfig{TargetLocaleIds: []string{"missing"}}},
			&pb.CreateTaskRequest{Name: "Source as target", Type: pb.TaskType_TASK_TYPE_AI_TRANSLATE, AiTranslate: &pb.AiTranslateConfig{SourceLocaleId: locale.ID, TargetLocaleIds: []string{locale.ID}}},
			&pb.CreateTaskRequest{Name: "No git settings", Type: pb.TaskType_TASK_TYPE_GIT_EXPORT},
			withGit(func(c *pb.GitExportConfig) { c.Repository = "app" }),
			withGit(func(c *pb.GitExportConfig) { c.Branch = "main" }),
			withGit(func(c *pb.GitExportConfig) { c.Branch = "bad branch" }),
			withGit(func(c *pb.GitExportConfig) { c.Branch = "a..b" }),
			withGit(func(c *pb.GitExportConfig) { c.ApiUrl = "ftp://github.example" }),
			withGit(func(c *pb.GitExportConfig) { c.Files = nil }),
			withGit(func(c *pb.GitExportConfig) { c.Files[0].Path = "lang/en.json" }),
			withGit(func(c *pb.GitExportConfig) { c.Files[0].Path = "../{locale}.json" }),
			withGit(func(c *pb.GitExportConfig) { c.Files[0].Path = "/lang/{locale}.json" }),
			withGit(func(c *pb.GitExportConfig) { c.Files[0].Path = "lang//{locale}.json" }),
			withGit(func(c *pb.GitExportConfig) { c.Files[0].Format = pb.ExportFormat_EXPORT_FORMAT_UNSPECIFIED }),
			withGit(func(c *pb.GitExportConfig) { c.Files = append(c.Files, c.Files[0]) }),
			withGit(func(c *pb.GitExportConfig) { c.ScopeIds = []string{"missing"} }),
		)
	})
}

func TestUpdateTask(t *testing.T) {
	ctx, server, _, _ := setup(t)

	created, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
		Name: "Ship", Type: pb.TaskType_TASK_TYPE_GIT_EXPORT, Enabled: true, GitExport: gitExport("acme/app"), Token: "first",
		Events: []pb.TaskEvent{pb.TaskEvent_TASK_EVENT_TRANSLATIONS_APPROVED},
	}))
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	id := created.Msg.Task.Id
	update := func(t *testing.T, req *pb.UpdateTaskRequest) *pb.Task {
		t.Helper()

		req.Id = id
		if req.GitExport == nil {
			req.GitExport = gitExport("acme/app")
		}

		res, err := server.UpdateTask(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}

		return res.Msg.Task
	}
	storedSecret := func() string {
		var task models.Task

		server.db.First(&task, "id = ?", id)

		return task.Secret
	}

	t.Run("keeps the token unless one is given", func(t *testing.T) {
		before := storedSecret()

		task := update(t, &pb.UpdateTaskRequest{Name: "Renamed", Enabled: true, GitExport: gitExport("acme/web")})
		if task.Name != "Renamed" || task.GitExport.Repository != "acme/web" || !task.HasToken || storedSecret() != before {
			t.Errorf("task = %+v", task)
		}

		if task := update(t, &pb.UpdateTaskRequest{Name: "Renamed", Enabled: true, Token: proto.String("")}); task.HasToken {
			t.Error("an empty token did not remove the stored one")
		}
	})

	t.Run("disabling drops runs queued by schedule and events", func(t *testing.T) {
		later := time.Now().Add(time.Hour)
		event := models.TaskRun{TaskID: id, Trigger: models.TriggerEvent, Status: models.RunQueued, RunAfter: later}
		server.db.Create(&event)

		update(t, &pb.UpdateTaskRequest{Name: "Renamed", Enabled: false})

		server.db.First(&event, "id = ?", event.ID)

		if event.Status != models.RunCanceled {
			t.Errorf("event run = %+v, want canceled", event)
		}
	})

	t.Run("can't change a git export into a machine translation", func(t *testing.T) {
		_, err := server.UpdateTask(ctx, connect.NewRequest(&pb.UpdateTaskRequest{Id: id, Name: "Ship"}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("update without git settings: error = %v", err)
		}
	})
}

func TestRuns(t *testing.T) {
	ctx, server, _, wakes := setup(t)

	created, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
		Name: "Ship", Type: pb.TaskType_TASK_TYPE_GIT_EXPORT, GitExport: gitExport("acme/app"),
	}))
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	id := created.Msg.Task.Id

	first, err := server.RunTask(ctx, connect.NewRequest(&pb.RunTaskRequest{Id: id}))
	if err != nil {
		t.Fatalf("RunTask() error = %v", err)
	}

	second, err := server.RunTask(ctx, connect.NewRequest(&pb.RunTaskRequest{Id: id}))
	if err != nil || second.Msg.Run.Id != first.Msg.Run.Id {
		t.Fatalf("second RunTask() = %v, %v, want the queued run", second, err)
	}

	if run := first.Msg.Run; run.Status != pb.TaskRunStatus_TASK_RUN_STATUS_QUEUED || run.Trigger != pb.TaskTrigger_TASK_TRIGGER_MANUAL || run.TriggeredBy == nil || *wakes != 2 {
		t.Errorf("run = %+v, wakes = %d", run, *wakes)
	}

	server.db.Model(&models.TaskRun{}).Where("id = ?", first.Msg.Run.Id).Updates(map[string]any{
		"status": models.RunSucceeded, "log": "12:00:00 Done\n", "summary": datatypes.JSON(`{"pull_request_url":"https://github.example/pull/1"}`),
	})

	listed, err := server.ListTasks(ctx, connect.NewRequest(&pb.ListTasksRequest{}))
	if err != nil || len(listed.Msg.Tasks) != 1 || listed.Msg.Tasks[0].LastRun.Summary.PullRequestUrl != "https://github.example/pull/1" {
		t.Fatalf("ListTasks() = %+v, %v", listed, err)
	}

	runs, err := server.ListTaskRuns(ctx, connect.NewRequest(&pb.ListTaskRunsRequest{TaskId: id}))
	if err != nil || len(runs.Msg.Runs) != 1 || runs.Msg.Runs[0].Log != "" || runs.Msg.Pagination.Total != 1 {
		t.Fatalf("ListTaskRuns() = %+v, %v, want one run without its log", runs, err)
	}

	if starter := runs.Msg.Runs[0].TriggeredByUser; starter == nil || starter.Id != *first.Msg.Run.TriggeredBy || starter.Name == "" {
		t.Errorf("listed run starter = %+v, want the user who started it", starter)
	}

	run, err := server.GetTaskRun(ctx, connect.NewRequest(&pb.GetTaskRunRequest{Id: first.Msg.Run.Id}))
	if err != nil || run.Msg.Run.Log != "12:00:00 Done\n" || run.Msg.Run.TriggeredByUser == nil {
		t.Fatalf("GetTaskRun() = %+v, %v, want its log and starter", run, err)
	}

	if _, err := server.CancelTaskRun(ctx, connect.NewRequest(&pb.CancelTaskRunRequest{Id: first.Msg.Run.Id})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("canceling an ended run: error = %v", err)
	}

	queued, _ := server.RunTask(ctx, connect.NewRequest(&pb.RunTaskRequest{Id: id}))

	canceled, err := server.CancelTaskRun(ctx, connect.NewRequest(&pb.CancelTaskRunRequest{Id: queued.Msg.Run.Id}))
	if err != nil || canceled.Msg.Run.Status != pb.TaskRunStatus_TASK_RUN_STATUS_CANCELED {
		t.Errorf("CancelTaskRun() = %+v, %v", canceled, err)
	}

	if _, err := server.DeleteTask(ctx, connect.NewRequest(&pb.DeleteTaskRequest{Id: id})); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	var left int64

	server.db.Model(&models.TaskRun{}).Where("task_id = ?", id).Count(&left)

	if left != 0 {
		t.Errorf("%d runs left after deleting the task", left)
	}
}

func TestTaskSettings(t *testing.T) {
	ctx, server, _, wakes := setup(t)
	t.Cleanup(func() { server.db.Exec(`DELETE FROM settings`) })

	got, err := server.GetTaskSettings(ctx, connect.NewRequest(&pb.GetTaskSettingsRequest{}))
	if err != nil || got.Msg.Settings.Paused || got.Msg.Settings.UpdatedAt != nil {
		t.Fatalf("GetTaskSettings() = %+v, %v, want running and never changed", got, err)
	}

	paused, err := server.UpdateTaskSettings(ctx, connect.NewRequest(&pb.UpdateTaskSettingsRequest{Paused: true}))
	if err != nil || !paused.Msg.Settings.Paused || paused.Msg.Settings.UpdatedAt == nil || *wakes != 0 {
		t.Fatalf("pausing = %+v, %v, wakes = %d", paused, err, *wakes)
	}

	resumed, err := server.UpdateTaskSettings(ctx, connect.NewRequest(&pb.UpdateTaskSettingsRequest{Paused: false}))
	if err != nil || resumed.Msg.Settings.Paused || *wakes != 1 {
		t.Errorf("resuming = %+v, %v, wakes = %d, want running and the runner woken", resumed, err, *wakes)
	}

	var setting models.Setting

	server.db.First(&setting, "key = ?", models.SettingTasksPaused)

	if setting.UpdatedBy == nil {
		t.Error("the setting doesn't say who changed it")
	}
}

func TestCheckTaskAccess(t *testing.T) {
	ctx, server, fake, _ := setup(t)

	created, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
		Name: "Ship", Type: pb.TaskType_TASK_TYPE_GIT_EXPORT, GitExport: gitExport(fake.Repository), Token: fake.Token,
	}))
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	check := func(req *pb.CheckTaskAccessRequest) *pb.CheckTaskAccessResponse {
		t.Helper()

		res, err := server.CheckTaskAccess(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("CheckTaskAccess() error = %v", err)
		}

		return res.Msg
	}

	if res := check(&pb.CheckTaskAccessRequest{Id: created.Msg.Task.Id}); !res.Ok {
		t.Errorf("saved settings: %+v", res)
	}

	if res := check(&pb.CheckTaskAccessRequest{Id: created.Msg.Task.Id, Token: proto.String("wrong")}); res.Ok || !strings.Contains(res.Error, "Bad credentials") {
		t.Errorf("wrong token: %+v", res)
	}

	unsaved := gitExport(fake.Repository)
	unsaved.BaseBranch = "develop"

	if res := check(&pb.CheckTaskAccessRequest{GitExport: unsaved, Token: proto.String(fake.Token)}); res.Ok || !strings.Contains(res.Error, "develop") {
		t.Errorf("missing base branch: %+v", res)
	}

	if res := check(&pb.CheckTaskAccessRequest{GitExport: gitExport(fake.Repository)}); res.Ok || !strings.Contains(res.Error, "token is required") {
		t.Errorf("no token: %+v", res)
	}
}
