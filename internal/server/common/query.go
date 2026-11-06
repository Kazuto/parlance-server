package common

import "gorm.io/gorm"

// Exists reports whether the query matches any row, e.g.
// Exists(db.Model(&models.Role{}).Where("name = ?", name)).
func Exists(query *gorm.DB) (bool, error) {
	var count int64
	if err := query.Limit(1).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}
