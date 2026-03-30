package converter

import (
	"github.com/kazuto/parlance-server/internal/models"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
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

	return &pb.Terminology{
		Id:          t.ID,
		Term:        t.Term,
		Description: t.Description,
		Definitions: definitions,
		CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
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

func DefinitionToProto(d *models.Definition) *pb.Definition {
	if d == nil {
		return nil
	}

	createdBy := ""
	if d.CreatedBy != nil {
		createdBy = *d.CreatedBy
	}

	updatedBy := ""
	if d.UpdatedBy != nil {
		updatedBy = *d.UpdatedBy
	}

	return &pb.Definition{
		Id:            d.ID,
		TerminologyId: d.TerminologyID,
		LocaleId:      d.LocaleID,
		Translation:   d.Translation,
		CreatedAt:     d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy:     createdBy,
		UpdatedBy:     updatedBy,
	}
}

func DefinitionHistoryToProto(h *models.DefinitionHistory) *pb.DefinitionHistory {
	if h == nil {
		return nil
	}

	userID := ""
	if h.UserID != nil {
		userID = *h.UserID
	}

	return &pb.DefinitionHistory{
		Id:            h.ID,
		DefinitionId:  h.DefinitionID,
		LocaleId:      h.LocaleID,
		TerminologyId: h.TerminologyID,
		UserId:        userID,
		Translation:   h.Translation,
		Action:        h.Action,
		ChangedAt:     h.ChangedAt.Format("2006-01-02T15:04:05Z07:00"),
		User:          UserToProto(h.User),
	}
}

func DefinitionHistoriesToProto(histories []models.DefinitionHistory) []*pb.DefinitionHistory {
	result := make([]*pb.DefinitionHistory, len(histories))
	for i, history := range histories {
		result[i] = DefinitionHistoryToProto(&history)
	}
	return result
}
