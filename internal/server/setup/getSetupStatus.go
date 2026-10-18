package setup

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
)

// GetSetupStatus checks if the application needs initial setup
func (s *Server) GetSetupStatus(
	ctx context.Context,
	req *connect.Request[pb.GetSetupStatusRequest],
) (*connect.Response[pb.GetSetupStatusResponse], error) {
	// Check if any users exist
	var count int64
	if err := s.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to check users")
	}

	// Setup is needed if no users exist
	needsSetup := count == 0

	return connect.NewResponse(&pb.GetSetupStatusResponse{
		NeedsSetup: needsSetup,
	}), nil
}
