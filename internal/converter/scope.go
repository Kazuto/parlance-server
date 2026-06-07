package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// ScopeToProto converts a database Scope model to protobuf
func ScopeToProto(s *models.Scope) *pb.Scope {
	if s == nil {
		return nil
	}

	deletedAt := ""
	if s.DeletedAt.Valid {
		deletedAt = s.DeletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &pb.Scope{
		Id:          s.ID,
		Name:        s.Name,
		Slug:        s.Slug,
		Description: s.Description,
		Color:       s.Color,
		CreatedAt:   s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		DeletedAt:   deletedAt,
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
