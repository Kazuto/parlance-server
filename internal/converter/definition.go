package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

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

	deletedAt := ""
	if d.DeletedAt.Valid {
		deletedAt = d.DeletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &pb.Definition{
		Id:            d.ID,
		TerminologyId: d.TerminologyID,
		LocaleId:      d.LocaleID,
		Translation:   d.Translation,
		CreatedAt:     d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		DeletedAt:     deletedAt,
		CreatedBy:     createdBy,
		UpdatedBy:     updatedBy,
	}
}

func DefinitionsToProto(definitions []models.Definition) []*pb.Definition {
	result := make([]*pb.Definition, len(definitions))
	for i, def := range definitions {
		result[i] = DefinitionToProto(&def)
	}
	return result
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
