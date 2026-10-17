package user

import (
	"bytes"
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
)

// UpdateUser updates an existing user
func (s *Server) UpdateUser(
	ctx context.Context,
	req *connect.Request[pb.UpdateUserRequest],
) (*connect.Response[pb.UpdateUserResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateUser); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user id is required"))
	}
	if req.Msg.Email != nil && req.Msg.GetEmail() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}
	if req.Msg.Name != nil && req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}
	if req.Msg.Password != nil && req.Msg.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password must not be empty"))
	}
	if len(req.Msg.GetAvatar()) > 0 && req.Msg.GetAvatarMime() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("avatar mime type is required"))
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	if req.Msg.Email != nil {
		user.Email = req.Msg.GetEmail()
	}

	if req.Msg.Name != nil {
		user.Name = req.Msg.GetName()
	}

	if req.Msg.Locale != nil {
		if locale := req.Msg.GetLocale(); locale != "" {
			user.Locale = &locale
		} else {
			user.Locale = nil
		}
	}

	if req.Msg.Password != nil {
		hashedPassword, err := auth.HashPassword(req.Msg.GetPassword())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password: %w", err))
		}

		user.PasswordHash = hashedPassword
	}

	if req.Msg.Avatar != nil {
		if len(req.Msg.Avatar) == 0 {
			user.Avatar = nil
		} else {
			url, err := s.uploadAvatar(ctx, user.ID, req.Msg.Avatar, req.Msg.GetAvatarMime())
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to upload avatar: %w", err))
			}

			user.Avatar = &url
		}
	}

	var roles []models.Role
	if req.Msg.Roles != nil {
		var err error
		roles, err = s.GetRoles(req.Msg.Roles.GetIds())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to get roles: %w", err))
		}
	}

	// Save only adds many2many links, so a sent selection replaces them explicitly.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Roles").Save(&user).Error; err != nil {
			return err
		}
		if req.Msg.Roles != nil {
			return tx.Model(&user).Association("Roles").Replace(roles)
		}
		return nil
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update user: %w", err))
	}

	// Load roles for response
	s.db.Preload("Roles.Permissions").First(&user, "id = ?", user.ID)

	return connect.NewResponse(&pb.UpdateUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}

// Upload to S3
func (s *Server) uploadAvatar(ctx context.Context, userID string, data []byte, contentType string) (string, error) {
	key := fmt.Sprintf("avatars/%s", userID)

	_, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s/%s/%s", s.endpoint, s.bucket, key), nil
}
