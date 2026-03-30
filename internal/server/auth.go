package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
)

// AuthServer implements the AuthService
type AuthServer struct {
	db     *database.DB
	config *config.Config
}

// NewAuthServer creates a new AuthServer
func NewAuthServer(db *database.DB, cfg *config.Config) parlancev1connect.AuthServiceHandler {
	return &AuthServer{
		db:     db,
		config: cfg,
	}
}

// Register creates a new user account
func (s *AuthServer) Register(
	ctx context.Context,
	req *connect.Request[pb.RegisterRequest],
) (*connect.Response[pb.RegisterResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}

	if req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password is required"))
	}

	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	// Check if user already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Msg.Email).First(&existingUser).Error; err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("user with this email already exists"))
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password: %w", err))
	}

	// Create user
	user := &models.User{
		Email:        req.Msg.Email,
		PasswordHash: hashedPassword,
		Name:         req.Msg.Name,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user: %w", err))
	}

	// Assign default viewer role
	var viewerRole models.Role
	if err := s.db.Where("name = ?", "viewer").First(&viewerRole).Error; err == nil {
		s.db.Model(user).Association("Roles").Append(&viewerRole)
	}

	// Load roles for response
	s.db.Preload("Roles.Permissions").First(user, "id = ?", user.ID)

	// Generate tokens
	accessExpiry, _ := time.ParseDuration(s.config.JWT.Expiry)
	refreshExpiry := accessExpiry * 7 // Refresh token valid for 7x access token duration

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate tokens: %w", err))
	}

	return connect.NewResponse(&pb.RegisterResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(user),
	}), nil
}

// Login authenticates a user and returns tokens
func (s *AuthServer) Login(
	ctx context.Context,
	req *connect.Request[pb.LoginRequest],
) (*connect.Response[pb.LoginResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}

	if req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password is required"))
	}

	// Find user
	var user models.User
	if err := s.db.Preload("Roles.Permissions").Where("email = ?", req.Msg.Email).First(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid email or password"))
	}

	// Check password
	if err := auth.CheckPassword(req.Msg.Password, user.PasswordHash); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid email or password"))
	}

	// Generate tokens
	accessExpiry, _ := time.ParseDuration(s.config.JWT.Expiry)
	refreshExpiry := accessExpiry * 7

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate tokens: %w", err))
	}

	return connect.NewResponse(&pb.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(&user),
	}), nil
}

// RefreshToken generates new tokens from a refresh token
func (s *AuthServer) RefreshToken(
	ctx context.Context,
	req *connect.Request[pb.RefreshTokenRequest],
) (*connect.Response[pb.RefreshTokenResponse], error) {
	if req.Msg.RefreshToken == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("refresh token is required"))
	}

	// Validate refresh token
	claims, err := auth.ValidateToken(req.Msg.RefreshToken, s.config.JWT.Secret)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
	}

	// Get user
	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Generate new tokens
	accessExpiry, _ := time.ParseDuration(s.config.JWT.Expiry)
	refreshExpiry := accessExpiry * 7

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate tokens: %w", err))
	}

	return connect.NewResponse(&pb.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(&user),
	}), nil
}

// Logout invalidates the current session
func (s *AuthServer) Logout(
	ctx context.Context,
	req *connect.Request[pb.LogoutRequest],
) (*connect.Response[pb.LogoutResponse], error) {
	// In a stateless JWT system, logout is typically handled client-side by discarding the token
	// For a more robust solution, implement token blacklisting with Redis

	return connect.NewResponse(&pb.LogoutResponse{}), nil
}

// GetCurrentUser returns the currently authenticated user
func (s *AuthServer) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[pb.GetCurrentUserRequest],
) (*connect.Response[pb.GetCurrentUserResponse], error) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := ctx.Value("user_id").(string)
	if !ok || userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	return connect.NewResponse(&pb.GetCurrentUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
