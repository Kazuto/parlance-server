package setup

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetSetupStatus checks if the application needs initial setup
func (s *Server) GetSetupStatus(
	ctx context.Context,
	req *connect.Request[pb.GetSetupStatusRequest],
) (*connect.Response[pb.GetSetupStatusResponse], error) {
	// Check if any users exist
	var count int64
	if err := s.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check users: %w", err))
	}

	// Setup is needed if no users exist
	needsSetup := count == 0

	return connect.NewResponse(&pb.GetSetupStatusResponse{
		NeedsSetup: needsSetup,
	}), nil
}
