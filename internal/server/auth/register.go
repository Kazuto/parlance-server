package auth

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// Register creates a new user account
func (s *Server) Register(
	ctx context.Context,
	req *connect.Request[pb.RegisterRequest],
) (*connect.Response[pb.RegisterResponse], error) {
	user, err := common.CreateUser(ctx, s.db.DB, common.NewUser{
		Email:       req.Msg.Email,
		Name:        req.Msg.Name,
		Password:    req.Msg.Password,
		DefaultRole: "viewer",
	})
	if err != nil {
		return nil, err
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RegisterResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(user),
	}), nil
}
