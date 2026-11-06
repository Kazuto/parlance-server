package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
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

// formatTime formats an optional time as RFC 3339, or returns "" without one
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339)
}

// formatPreciseTime formats an optional time as RFC 3339 with fractions of a second,
// for times clients filter by, or returns "" without one
func formatPreciseTime(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339Nano)
}

// ReviewStatusToProto maps a stored review status to its proto value
func ReviewStatusToProto(status string) pb.ReviewStatus {
	switch status {
	case models.StatusDraft:
		return pb.ReviewStatus_REVIEW_STATUS_DRAFT
	case models.StatusApproved:
		return pb.ReviewStatus_REVIEW_STATUS_APPROVED
	default:
		return pb.ReviewStatus_REVIEW_STATUS_UNSPECIFIED
	}
}
