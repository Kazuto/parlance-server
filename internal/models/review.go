package models

import "time"

// Review statuses of a translation or definition
const (
	StatusDraft    = "draft"
	StatusApproved = "approved"
)

// History actions besides "created", "updated" and "deleted"
const (
	ActionApproved = "approved"
	ActionRejected = "rejected"
)

// Review holds where a translation stands in review. New rows are drafts; changing
// the text makes a row a draft again, and a reviewer approves or rejects it.
type Review struct {
	Status     string     `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	ReviewedBy *string    `gorm:"type:uuid" json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	// Why the last reviewer sent the text back; cleared once it is approved.
	ReviewComment string `gorm:"type:text;not null;default:''" json:"review_comment"`
}

// MarkDraft makes the text a draft after it changed. The last review stays, so a
// rejection's comment shows until the text is approved.
func (r *Review) MarkDraft() {
	r.Status = StatusDraft
}

// Approve accepts the text
func (r *Review) Approve(reviewer *string, at time.Time) {
	r.Status = StatusApproved
	r.ReviewedBy = reviewer
	r.ReviewedAt = &at
	r.ReviewComment = ""
}

// Reject sends the text back as a draft, saying what should change
func (r *Review) Reject(reviewer *string, at time.Time, comment string) {
	r.Status = StatusDraft
	r.ReviewedBy = reviewer
	r.ReviewedAt = &at
	r.ReviewComment = comment
}

// ReviewState gives access to the review of a row that embeds it
func (r *Review) ReviewState() *Review {
	return r
}
