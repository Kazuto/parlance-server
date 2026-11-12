package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// ReleaseToProto converts a database Release model to protobuf
func ReleaseToProto(r *models.Release) *pb.Release {
	if r == nil {
		return nil
	}

	return &pb.Release{
		Id:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		ReleaseAt:   optionalTime(r.ReleaseAt),
		ReleasedAt:  optionalTime(r.ReleasedAt),
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
	}
}

// ReleasesToProto converts a slice of Release models to protobuf
func ReleasesToProto(releases []models.Release) []*pb.Release {
	result := make([]*pb.Release, len(releases))
	for i, release := range releases {
		result[i] = ReleaseToProto(&release)
	}

	return result
}
