package user

import (
	"errors"

	"github.com/kazuto/parlance-server/internal/models"
)

func (s *Server) GetRoles(roleIDs []string) ([]models.Role, error) {
	var roles []models.Role
	s.db.Where("id IN ?", roleIDs).Find(&roles)

	if len(roles) != len(roleIDs) {
		return nil, errors.New("one or more role IDs are invalid")
	}

	return roles, nil
}
