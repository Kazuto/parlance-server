package converter

import (
	"time"

	"gorm.io/gorm"
)

// derefString returns the pointed-to string, or "" for nil
func derefString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

// formatDeletedAt formats a soft-delete time as RFC 3339, or returns "" for a live row
func formatDeletedAt(deletedAt gorm.DeletedAt) string {
	if !deletedAt.Valid {
		return ""
	}

	return deletedAt.Time.Format(time.RFC3339)
}
