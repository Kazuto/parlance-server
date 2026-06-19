package role

import (
	"errors"

	"github.com/kazuto/parlance-server/internal/models"
)

func (s *Server) GetPermissions(permissionIDs []string) ([]models.Permission, error) {
	var permissions []models.Permission
	s.db.Where("id IN ?", permissionIDs).Find(&permissions)

	if len(permissions) != len(permissionIDs) {
		return nil, errors.New("one or more permission IDs are invalid")
	}

	return permissions, nil
}
