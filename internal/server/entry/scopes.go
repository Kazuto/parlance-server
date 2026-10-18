package entry

import (
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

func resolveScopes(db *gorm.DB, ids []string) ([]models.Scope, error) {
	unique := map[string]bool{}
	for _, id := range ids {
		unique[id] = true
	}
	var scopes []models.Scope
	if len(unique) == 0 {
		return scopes, nil
	}
	if err := db.Where("id IN ?", ids).Find(&scopes).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to find scopes")
	}
	if len(scopes) != len(unique) {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "one or more scopes do not exist")
	}
	return scopes, nil
}
