package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// TagToProto converts a database Tag model to protobuf
func TagToProto(t *models.Tag) *pb.Tag {
	if t == nil {
		return nil
	}

	return &pb.Tag{
		Id:        t.ID,
		ScopeId:   t.ScopeID,
		Name:      t.Name,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}

// TagsToProto converts a slice of Tag models to protobuf
func TagsToProto(tags []models.Tag) []*pb.Tag {
	result := make([]*pb.Tag, len(tags))
	for i, tag := range tags {
		result[i] = TagToProto(&tag)
	}

	return result
}
