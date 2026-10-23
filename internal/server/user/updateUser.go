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
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
)

// UpdateUser updates an existing user
func (s *Server) UpdateUser(
	ctx context.Context,
	req *connect.Request[pb.UpdateUserRequest],
) (*connect.Response[pb.UpdateUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	if req.Msg.Email != nil && req.Msg.GetEmail() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "email is required")
	}

	if req.Msg.Name != nil && req.Msg.GetName() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	if req.Msg.Password != nil && req.Msg.GetPassword() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "password must not be empty")
	}

	if len(req.Msg.GetAvatar()) > 0 && req.Msg.GetAvatarMime() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "avatar mime type is required")
	}

	user, err := common.FindByID[models.User](s.db.WithContext(ctx), req.Msg.Id, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
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
			return nil, errors.Internal(errors.ErrInternal, "failed to hash password")
		}

		user.PasswordHash = hashedPassword
	}

	if req.Msg.Avatar != nil {
		if len(req.Msg.Avatar) == 0 {
			user.Avatar = nil
		} else {
			url, err := s.uploadAvatar(ctx, user.ID, req.Msg.Avatar, req.Msg.GetAvatarMime())
			if err != nil {
				return nil, errors.Internal(errors.ErrInternal, "failed to upload avatar")
			}

			user.Avatar = &url
		}
	}

	var roles []models.Role

	if req.Msg.Roles != nil {
		var err error

		roles, err = common.ResolveByIDs[models.Role](s.db.WithContext(ctx), req.Msg.Roles.GetIds(), errors.ErrRoleNotFound, "one or more roles do not exist")
		if err != nil {
			return nil, err
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
		return nil, errors.Internal(errors.ErrInternal, "failed to update user")
	}

	// Load roles for response
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", user.ID).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load updated user")
	}

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
