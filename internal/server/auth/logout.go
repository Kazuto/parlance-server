package auth

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// Logout invalidates the current session
func (s *Server) Logout(
	ctx context.Context,
	req *connect.Request[pb.LogoutRequest],
) (*connect.Response[pb.LogoutResponse], error) {
	// In a stateless JWT system, logout is typically handled client-side by discarding the token
	// For a more robust solution, implement token blacklisting with Redis

	return connect.NewResponse(&pb.LogoutResponse{}), nil
}
