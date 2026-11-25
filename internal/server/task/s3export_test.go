package task

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/objectstore/objectstoretest"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func s3Export(fake *objectstoretest.S3) *pb.S3ExportConfig {
	return &pb.S3ExportConfig{
		Endpoint:     fake.Server.URL,
		Bucket:       fake.Bucket,
		AccessKeyId:  fake.AccessKeyID,
		PathStyle:    true,
		ManifestPath: "i18n/manifest.json",
		Files:        []*pb.GitExportFile{{Format: pb.ExportFormat_EXPORT_FORMAT_JSON, Path: "i18n/{locale}.{hash}.json"}},
	}
}

func TestCreateS3Export(t *testing.T) {
	ctx, server, _, _ := setup(t)
	fake := objectstoretest.NewS3(t)

	t.Run("saves the settings with the secret key encrypted", func(t *testing.T) {
		config := s3Export(fake)
		config.Endpoint = " " + fake.Server.URL + "/ "
		config.HashLength = 16

		res, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
			Name: "Upload", Type: pb.TaskType_TASK_TYPE_S3_EXPORT, S3Export: config, Token: fake.SecretAccessKey,
		}))
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		task := res.Msg.Task
		if task.Type != pb.TaskType_TASK_TYPE_S3_EXPORT || !task.HasToken || task.GitExport != nil || task.S3Export == nil {
			t.Fatalf("task = %+v", task)
		}

		if got := task.S3Export; got.Endpoint != fake.Server.URL || got.Bucket != fake.Bucket || !got.PathStyle ||
			got.HashLength != 16 || got.ManifestPath != "i18n/manifest.json" || len(got.Files) != 1 {
			t.Errorf("config = %+v", got)
		}

		var stored models.Task

		server.db.First(&stored, "id = ?", task.Id)

		if plain, err := server.box.Decrypt(stored.Secret); err != nil || plain != fake.SecretAccessKey {
			t.Errorf("stored secret = %q, %v", plain, err)
		}
	})

	t.Run("rejects invalid settings", func(t *testing.T) {
		withS3 := func(change func(*pb.S3ExportConfig)) *pb.CreateTaskRequest {
			config := s3Export(fake)
			change(config)

			return &pb.CreateTaskRequest{Name: "Upload", Type: pb.TaskType_TASK_TYPE_S3_EXPORT, S3Export: config}
		}

		testutil.ExpectInvalidArgument(t, ctx, server.CreateTask,
			&pb.CreateTaskRequest{Name: "No settings", Type: pb.TaskType_TASK_TYPE_S3_EXPORT},
			withS3(func(c *pb.S3ExportConfig) { c.Endpoint = "ftp://storage.example" }),
			withS3(func(c *pb.S3ExportConfig) { c.Endpoint = "https://storage.example?bucket=x" }),
			withS3(func(c *pb.S3ExportConfig) { c.Region = "eu central" }),
			withS3(func(c *pb.S3ExportConfig) { c.Bucket = "" }),
			withS3(func(c *pb.S3ExportConfig) { c.Bucket = "my/bucket" }),
			withS3(func(c *pb.S3ExportConfig) { c.AccessKeyId = " " }),
			withS3(func(c *pb.S3ExportConfig) { c.ManifestPath = "" }),
			withS3(func(c *pb.S3ExportConfig) { c.ManifestPath = "/i18n/manifest.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.ManifestPath = "i18n/../manifest.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.ManifestPath = "i18n/{locale}.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.HashLength = 7 }),
			withS3(func(c *pb.S3ExportConfig) { c.HashLength = 65 }),
			withS3(func(c *pb.S3ExportConfig) { c.Files = nil }),
			withS3(func(c *pb.S3ExportConfig) { c.Files[0].Path = "i18n/{locale}.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.Files[0].Path = "i18n/{hash}.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.Files[0].Path = "i18n/./{locale}.{hash}.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.Files[0].Path = strings.Repeat("a", 900) + "/{locale}.{hash}.json" }),
			withS3(func(c *pb.S3ExportConfig) { c.LocaleIds = []string{"missing"} }),
		)
	})
}

func TestCheckS3ExportAccess(t *testing.T) {
	ctx, server, _, _ := setup(t)
	fake := objectstoretest.NewS3(t)

	created, err := server.CreateTask(ctx, connect.NewRequest(&pb.CreateTaskRequest{
		Name: "Upload", Type: pb.TaskType_TASK_TYPE_S3_EXPORT, S3Export: s3Export(fake), Token: fake.SecretAccessKey,
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

	if calls := fake.Calls(); !strings.Contains(strings.Join(calls, ","), "PUT i18n/.parlance-check") {
		t.Errorf("calls = %v, want a write next to the manifest", calls)
	}

	other := s3Export(fake)
	other.Bucket = "other"

	if res := check(&pb.CheckTaskAccessRequest{Id: created.Msg.Task.Id, S3Export: other}); res.Ok || !strings.Contains(res.Error, `"other" does not exist`) {
		t.Errorf("missing bucket: %+v", res)
	}

	if res := check(&pb.CheckTaskAccessRequest{S3Export: s3Export(fake)}); res.Ok || !strings.Contains(res.Error, "secret access key is required") {
		t.Errorf("no secret key: %+v", res)
	}

	fake.SetReadOnly(true)

	if res := check(&pb.CheckTaskAccessRequest{S3Export: s3Export(fake), Token: proto.String(fake.SecretAccessKey)}); res.Ok || !strings.Contains(res.Error, "denied access") {
		t.Errorf("read-only key: %+v", res)
	}

	fake.SetReadOnly(false)

	testutil.ExpectInvalidArgument(t, ctx, server.CheckTaskAccess,
		&pb.CheckTaskAccessRequest{},
		&pb.CheckTaskAccessRequest{Id: created.Msg.Task.Id, GitExport: gitExport("acme/app")},
		&pb.CheckTaskAccessRequest{GitExport: gitExport("acme/app"), S3Export: s3Export(fake)},
	)
}
