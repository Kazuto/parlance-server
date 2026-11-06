package models

import (
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

// demoReview approves the demo text, except French, which is left for review.
func demoReview(localeCode, reviewerID string) models.Review {
	review := models.Review{Status: models.StatusDraft}
	if localeCode != "fr" {
		review.Approve(&reviewerID, time.Now())
	}

	return review
}
