package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// ScopeToProto converts a database Scope model to protobuf
func ScopeToProto(s *models.Scope) *pb.Scope {
	if s == nil {
		return nil
	}

	return &pb.Scope{
		Id:          s.ID,
		Name:        s.Name,
		Slug:        s.Slug,
		Description: s.Description,
		Color:       s.Color,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
		DeletedAt:   formatDeletedAt(s.DeletedAt),
	}
}

// ScopesToProto converts a slice of Scope models to protobuf
func ScopesToProto(scopes []models.Scope) []*pb.Scope {
	result := make([]*pb.Scope, len(scopes))
	for i, scope := range scopes {
		result[i] = ScopeToProto(&scope)
	}

	return result
}
