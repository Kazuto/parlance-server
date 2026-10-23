package common

import (
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// DefaultLocale loads the default locale, or returns nil when none is set
func DefaultLocale(db *gorm.DB) (*models.Locale, error) {
	var locales []models.Locale
	if err := db.Where("is_default = ?", true).Limit(1).Find(&locales).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load default locale")
	}

	if len(locales) == 0 {
		return nil, nil
	}

	return &locales[0], nil
}
