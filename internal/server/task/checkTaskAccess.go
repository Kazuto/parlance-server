package task

import (
	"context"
	"path"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/objectstore"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// checkTimeout limits how long CheckTaskAccess waits for the host or storage.
const checkTimeout = 20 * time.Second

// CheckTaskAccess checks that a git export's repository and base branch can be read
// and pushed to, or that an S3 export's bucket can be written next to its manifest,
// with the given settings and token or else the saved ones.
func (s *Server) CheckTaskAccess(
	ctx context.Context,
	req *connect.Request[pb.CheckTaskAccessRequest],
) (*connect.Response[pb.CheckTaskAccessResponse], error) {
	var task models.Task

	if req.Msg.Id != "" {
		var err error
		if task, err = s.findTask(req.Msg.Id); err != nil {
			return nil, err
		}

		if !models.IsExportType(task.Type) {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "only exports have access to check")
		}
	}

	taskType, err := checkedType(req.Msg, task.Type)
	if err != nil {
		return nil, err
	}

	secretName := "token"
	if taskType == models.TaskTypeS3Export {
		secretName = "secret access key"
	}

	token := req.Msg.GetToken()

	if req.Msg.Token == nil && task.Secret != "" {
		if token, err = s.box.Decrypt(task.Secret); err != nil {
			return failedCheck("the saved " + secretName + " can't be read, e.g. because ENCRYPTION_KEY changed; enter it again"), nil
		}
	}

	if taskType == models.TaskTypeS3Export {
		return s.checkBucketAccess(ctx, req.Msg.S3Export, task, token)
	}

	return s.checkRepositoryAccess(ctx, req.Msg.GitExport, task, token)
}

// checkedType is the type of export to check: the saved task's, which settings given
// must match, or else that of the settings given.
func checkedType(msg *pb.CheckTaskAccessRequest, savedType string) (string, error) {
	var given string

	switch {
	case msg.GitExport != nil && msg.S3Export != nil:
		return "", errors.InvalidInput(errors.ErrInvalidInput, "give either git or S3 export settings")
	case msg.GitExport != nil:
		given = models.TaskTypeGitExport
	case msg.S3Export != nil:
		given = models.TaskTypeS3Export
	}

	if savedType == "" && given == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "git or S3 export settings are required")
	}

	if savedType != "" && given != "" && savedType != given {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "the settings are for another type of export than the task")
	}

	if savedType != "" {
		return savedType, nil
	}

	return given, nil
}

func (s *Server) checkRepositoryAccess(ctx context.Context, in *pb.GitExportConfig, task models.Task, token string) (*connect.Response[pb.CheckTaskAccessResponse], error) {
	if in == nil {
		saved, err := task.GitExport()
		if err != nil {
			return nil, errors.InternalCause(err, "failed to read the task settings")
		}

		in = &pb.GitExportConfig{ApiUrl: saved.APIURL, Repository: saved.Repository, BaseBranch: saved.BaseBranch, Branch: saved.Branch}
	}

	// The pull request branch is checked only as a name here.
	config, err := checkRepository(in)
	if err != nil {
		return nil, err
	}

	if token == "" {
		return failedCheck("an access token is required"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	if err := s.newHost(config.APIURL, token).Check(ctx, config.Repository, config.BaseBranch); err != nil {
		return failedCheck(err.Error()), nil
	}

	return connect.NewResponse(&pb.CheckTaskAccessResponse{Ok: true}), nil
}

func (s *Server) checkBucketAccess(ctx context.Context, in *pb.S3ExportConfig, task models.Task, secretKey string) (*connect.Response[pb.CheckTaskAccessResponse], error) {
	if in == nil {
		saved, err := task.S3Export()
		if err != nil {
			return nil, errors.InternalCause(err, "failed to read the task settings")
		}

		in = converter.S3ExportConfigToProto(saved)
	}

	config, err := checkBucket(in)
	if err != nil {
		return nil, err
	}

	if secretKey == "" {
		return failedCheck("a secret access key is required"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	store := s.newStore(objectstore.Config{
		Endpoint:        config.Endpoint,
		Region:          config.Region,
		Bucket:          config.Bucket,
		AccessKeyID:     config.AccessKeyID,
		SecretAccessKey: secretKey,
		PathStyle:       config.PathStyle,
	})

	// The check writes next to the manifest, as keys may only write below a prefix.
	prefix := path.Dir(config.ManifestPath)
	if prefix == "." {
		prefix = ""
	}

	if err := store.Check(ctx, prefix); err != nil {
		return failedCheck(err.Error()), nil
	}

	return connect.NewResponse(&pb.CheckTaskAccessResponse{Ok: true}), nil
}

func failedCheck(message string) *connect.Response[pb.CheckTaskAccessResponse] {
	return connect.NewResponse(&pb.CheckTaskAccessResponse{Error: message})
}
