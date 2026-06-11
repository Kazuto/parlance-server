package user

import (
	"bytes"
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateUser updates an existing user
func (s *Server) UpdateUser(
	ctx context.Context,
	req *connect.Request[pb.UpdateUserRequest],
) (*connect.Response[pb.UpdateUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user id is required"))
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	if req.Msg.Email != "" {
		user.Email = req.Msg.Email
	}

	if req.Msg.Name != "" {
		user.Name = req.Msg.Name
	}

	if req.Msg.Locale != "" {
		user.Locale = &req.Msg.Locale
	}

	if req.Msg.Avatar != nil && req.Msg.AvatarMime != "" {
		url, err := s.uploadAvatar(ctx, user.ID, req.Msg.Avatar, req.Msg.AvatarMime)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to upload avatar: %w", err))
		}

		user.Avatar = &url
	}

	if err := s.db.Save(&user).Error; err != nil {
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
