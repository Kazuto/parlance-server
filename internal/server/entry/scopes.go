package entry

import (
	"fmt"

	"connectrpc.com/connect"
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
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find scopes: %w", err))
	}
	if len(scopes) != len(unique) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("one or more scopes do not exist"))
	}
	return scopes, nil
}
