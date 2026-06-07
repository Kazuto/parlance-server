package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

func TerminologyToProto(t *models.Terminology) *pb.Terminology {
	if t == nil {
		return nil
	}

	definitions := make([]*pb.Definition, len(t.Definitions))
	for i, def := range t.Definitions {
		definitions[i] = DefinitionToProto(&def)
	}

	createdBy := ""
	if t.CreatedBy != nil {
		createdBy = *t.CreatedBy
	}

	updatedBy := ""
	if t.UpdatedBy != nil {
		updatedBy = *t.UpdatedBy
	}

	deletedAt := ""
	if t.DeletedAt.Valid {
		deletedAt = t.DeletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &pb.Terminology{
		Id:          t.ID,
		Term:        t.Term,
		Description: t.Description,
		Definitions: definitions,
		CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		DeletedAt:   deletedAt,
		CreatedBy:   createdBy,
		UpdatedBy:   updatedBy,
	}
}

func TerminologiesToProto(terminologies []models.Terminology) []*pb.Terminology {
	result := make([]*pb.Terminology, len(terminologies))
	for i, term := range terminologies {
		result[i] = TerminologyToProto(&term)
	}
	return result
}
