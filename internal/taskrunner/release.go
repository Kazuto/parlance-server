package taskrunner

import (
	"context"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/release"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// releaseDue lets out the pending releases whose date came. Releases another server is
// handling are skipped.
func (r *Runner) releaseDue(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		var due []models.Release
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("released_at IS NULL AND release_at <= ?", now.UTC()).
			Order("release_at").
			Find(&due).Error; err != nil {
			return err
		}

		for i := range due {
			if err := release.Publish(tx, &due[i], now, nil); err != nil {
				return err
			}
		}

		return nil
	})
}
