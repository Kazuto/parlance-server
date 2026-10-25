// Package settings reads and writes the instance-wide settings admins change in the
// app, stored as JSON in the settings table.
package settings

import (
	"encoding/json"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Bool returns a yes-or-no setting and when it last changed, or fallback and the zero
// time while it was never set
func Bool(db *gorm.DB, key string, fallback bool) (bool, time.Time, error) {
	var rows []models.Setting
	if err := db.Where("key = ?", key).Limit(1).Find(&rows).Error; err != nil {
		return fallback, time.Time{}, err
	}

	if len(rows) == 0 {
		return fallback, time.Time{}, nil
	}

	var value bool
	if err := json.Unmarshal(rows[0].Value, &value); err != nil {
		return fallback, time.Time{}, err
	}

	return value, rows[0].UpdatedAt, nil
}

// SetBool saves a yes-or-no setting, crediting the user who changed it
func SetBool(db *gorm.DB, key string, value bool, by *string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
	}).Create(&models.Setting{Key: key, Value: encoded, UpdatedBy: by}).Error
}
