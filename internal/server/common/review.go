package common

import (
	"context"
	"strings"
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// Reviewable is a row with a review state and its own history
type Reviewable interface {
	ReviewState() *models.Review
	RecordReview(tx *gorm.DB, action string) error
}

// ReviewSelection picks rows by id, or by their parent (entry or terminology) and locale
type ReviewSelection struct {
	IDs       []string
	ParentIDs []string
	LocaleIDs []string
	// The column holding the parent id, e.g. "entry_id"
	ParentColumn string
}

// ValidateReview checks a review request before rows are loaded
func ValidateReview(selection ReviewSelection, decision pb.ReviewDecision, comment string) error {
	switch decision {
	case pb.ReviewDecision_REVIEW_DECISION_APPROVE:
	case pb.ReviewDecision_REVIEW_DECISION_REJECT:
		if strings.TrimSpace(comment) == "" {
			return errors.InvalidInput(errors.ErrInvalidInput, "a comment is required when rejecting")
		}
	default:
		return errors.InvalidInput(errors.ErrInvalidInput, "decision is required")
	}

	if len(selection.IDs) == 0 && len(selection.ParentIDs) == 0 {
		return errors.InvalidInput(errors.ErrInvalidInput, "nothing to review")
	}

	if len(selection.ParentIDs) > 0 && len(selection.LocaleIDs) == 0 {
		return errors.InvalidInput(errors.ErrInvalidInput, "locale ids are required with parent ids")
	}

	return nil
}

// ReviewRows loads the selected rows and approves or rejects them in one transaction,
// recording each review in their history. Rows that don't exist, and approved rows when
// approving, are skipped.
func ReviewRows[T any, P interface {
	*T
	Reviewable
}](ctx context.Context, db *gorm.DB, selection ReviewSelection, decision pb.ReviewDecision, comment string) ([]T, error) {
	if err := ValidateReview(selection, decision, comment); err != nil {
		return nil, err
	}

	var rows []T

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(new(T))

		ids := UniqueIDs(selection.IDs)
		parentIDs := UniqueIDs(selection.ParentIDs)

		switch {
		case len(ids) > 0 && len(parentIDs) > 0:
			query = query.Where("id IN ? OR ("+selection.ParentColumn+" IN ? AND locale_id IN ?)", ids, parentIDs, selection.LocaleIDs)
		case len(ids) > 0:
			query = query.Where("id IN ?", ids)
		default:
			query = query.Where(selection.ParentColumn+" IN ? AND locale_id IN ?", parentIDs, selection.LocaleIDs)
		}

		// Approving leaves approved rows as they are, so their reviewer and history stay.
		if decision == pb.ReviewDecision_REVIEW_DECISION_APPROVE {
			query = query.Where("status = ?", models.StatusDraft)
		}

		if err := query.Find(&rows).Error; err != nil {
			return err
		}

		reviewer := middleware.UserIDPtr(ctx)
		now := time.Now()
		action := models.ActionApproved

		if decision == pb.ReviewDecision_REVIEW_DECISION_REJECT {
			action = models.ActionRejected
		}

		// The review is not an edit, so it records its own history instead of "updated".
		save := tx.Session(&gorm.Session{SkipHooks: true})

		for i := range rows {
			row := P(&rows[i])
			review := row.ReviewState()

			if action == models.ActionApproved {
				review.Approve(reviewer, now)
			} else {
				review.Reject(reviewer, now, strings.TrimSpace(comment))
			}

			if err := save.Model(row).Select("status", "reviewed_by", "reviewed_at", "review_comment", "updated_at").Updates(row).Error; err != nil {
				return err
			}

			if err := row.RecordReview(tx, action); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to review")
	}

	return rows, nil
}
