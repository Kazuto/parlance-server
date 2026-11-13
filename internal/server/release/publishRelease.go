package release

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/middleware"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// PublishRelease lets a pending release out now, whatever its date
func (s *Server) PublishRelease(
	ctx context.Context,
	req *connect.Request[pb.PublishReleaseRequest],
) (*connect.Response[pb.PublishReleaseResponse], error) {
	db := s.db.WithContext(ctx)

	release, err := findRelease(db, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return Publish(tx, &release, time.Now(), middleware.UserIDPtr(ctx))
	}); err != nil {
		return nil, err
	}

	result, err := releaseToProto(db, release)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.PublishReleaseResponse{Release: result}), nil
}
