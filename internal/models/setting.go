package models

import (
	"time"

	"gorm.io/datatypes"
)

// Keys of settings
const (
	// SettingTasksPaused holds whether tasks are paused: nothing is scheduled or run.
	SettingTasksPaused = "tasks.paused"
)

// Setting is an instance-wide setting admins change in the app, stored as JSON.
type Setting struct {
	Key       string         `gorm:"type:varchar(100);primaryKey" json:"key"`
	Value     datatypes.JSON `gorm:"type:jsonb;not null" json:"value"`
	UpdatedAt time.Time      `json:"updated_at"`
	UpdatedBy *string        `gorm:"type:uuid" json:"updated_by,omitempty"`
}
