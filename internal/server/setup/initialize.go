package setup

import (
	"context"
	"log"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
)

// Initialize performs the initial setup of the application
func (s *Server) Initialize(
	ctx context.Context,
	req *connect.Request[pb.InitializeRequest],
) (*connect.Response[pb.InitializeResponse], error) {
	// Check if setup has already been done
	exists, err := common.Exists(s.db.WithContext(ctx).Model(&models.User{}))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to check users")
	}

	if exists {
		return nil, errors.FailedPrecondition(errors.ErrSetupAlreadyCompleted, "setup already completed")
	}

	// The server bootstraps the admin role with all permissions at startup.
	var adminRole models.Role
	if err := s.db.WithContext(ctx).Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to get admin role")
	}

	adminUser, err := common.CreateUser(ctx, s.db.DB, common.NewUser{
		Email:    req.Msg.AdminEmail,
		Name:     req.Msg.AdminName,
		Password: req.Msg.AdminPassword,
		Roles:    []models.Role{adminRole},
	})
	if err != nil {
		return nil, err
	}

	log.Printf("Admin user created: %s", adminUser.Email)

	return connect.NewResponse(&pb.InitializeResponse{
		Success: true,
		Message: "Setup completed successfully. You can now log in with your admin account.",
	}), nil
}
