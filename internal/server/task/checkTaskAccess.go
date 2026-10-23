package task

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// checkTimeout limits how long CheckTaskAccess waits for the host.
const checkTimeout = 20 * time.Second

// CheckTaskAccess checks that a git export's repository and base branch can be read
// and pushed to, with the given settings and token or else the saved ones.
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

		if task.Type != models.TaskTypeGitExport {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "only git exports have access to check")
		}
	}

	in := req.Msg.GitExport
	if in == nil && task.ID != "" {
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

	token := req.Msg.GetToken()

	if req.Msg.Token == nil && task.Secret != "" {
		if token, err = s.box.Decrypt(task.Secret); err != nil {
			return failedCheck("the saved token can't be read, e.g. because ENCRYPTION_KEY changed; enter it again"), nil
		}
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

func failedCheck(message string) *connect.Response[pb.CheckTaskAccessResponse] {
	return connect.NewResponse(&pb.CheckTaskAccessResponse{Error: message})
}
