package setup

import (
	"context"
	"log"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/auth"
	bootstrap "github.com/kazuto/parlance-server/internal/database/bootstrap/cmd"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
)

// Initialize performs the initial setup of the application
func (s *Server) Initialize(
	ctx context.Context,
	req *connect.Request[pb.InitializeRequest],
) (*connect.Response[pb.InitializeResponse], error) {
	// Check if setup has already been done
	var count int64
	if err := s.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to check users")
	}

	if count > 0 {
		return nil, errors.FailedPrecondition(errors.ErrSetupAlreadyCompleted, "setup already completed")
	}

	// Validate input
	if req.Msg.AdminEmail == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "admin email is required")
	}
	if req.Msg.AdminPassword == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "admin password is required")
	}
	if req.Msg.AdminName == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "admin name is required")
	}

	// Bootstrap permissions and roles
	log.Println("Running bootstrap...")
	if err := bootstrap.Execute(s.db.DB); err != nil {
		return nil, errors.Internal(errors.ErrSetupFailed, "bootstrap failed")
	}

	// Get the admin role
	var adminRole models.Role
	if err := s.db.Where("name = ?", "admin").Preload("Permissions").First(&adminRole).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to get admin role")
	}

	// Hash the password
	hashedPassword, err := auth.HashPassword(req.Msg.AdminPassword)
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to hash password")
	}

	// Create the admin user
	locale := "en"
	adminUser := &models.User{
		Email:        req.Msg.AdminEmail,
		PasswordHash: hashedPassword,
		Name:         req.Msg.AdminName,
		Locale:       &locale,
		Roles:        []models.Role{adminRole},
	}

	if err := s.db.Create(adminUser).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create admin user")
	}

	log.Printf("Admin user created: %s", adminUser.Email)

	return connect.NewResponse(&pb.InitializeResponse{
		Success: true,
		Message: "Setup completed successfully. You can now log in with your admin account.",
	}), nil
}
