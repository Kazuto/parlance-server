package common

import (
	"context"
	stderrors "errors"
	"reflect"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FindByID loads the row with the ID through query, which may carry Preload or Unscoped. A
// missing row becomes NotFound with the code and message; other failures are internal.
func FindByID[T any](query *gorm.DB, id, notFoundCode, notFoundMessage string) (T, error) {
	var row T
	if err := query.First(&row, "id = ?", id).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return row, errors.NotFound(notFoundCode, notFoundMessage)
		}

		return row, errors.InternalCause(err, "failed to load record")
	}

	return row, nil
}

// DeleteRow soft-deletes a loaded row, crediting the current user in updated_by first so the
// deletion's history names them.
func DeleteRow(ctx context.Context, db *gorm.DB, row any) error {
	setUpdatedBy(db, row, middleware.UserIDPtr(ctx))

	return db.WithContext(ctx).Delete(row).Error
}

// Restore undeletes the soft-deleted row with the ID, crediting the current user, and returns
// it loaded through query, which may carry Preload. Associations are left untouched.
func Restore[T any](ctx context.Context, query *gorm.DB, id, notFoundCode, notFoundMessage string) (T, error) {
	row, err := FindByID[T](query.WithContext(ctx).Unscoped(), id, notFoundCode, notFoundMessage)
	if err != nil {
		return row, err
	}

	columns := []any{"deleted_at"}
	if setUpdatedBy(query, &row, middleware.UserIDPtr(ctx)) {
		columns = append(columns, "updated_by")
	}

	reflect.ValueOf(&row).Elem().FieldByName("DeletedAt").Set(reflect.ValueOf(gorm.DeletedAt{}))

	tx := query.Session(&gorm.Session{NewDB: true}).WithContext(ctx).Unscoped()
	if err := tx.Model(&row).Select(columns[0], columns[1:]...).Omit(clause.Associations).Updates(&row).Error; err != nil {
		return row, errors.InternalCause(err, "failed to restore record")
	}

	return row, nil
}

// setUpdatedBy sets the row's UpdatedBy field when its model has one, and reports whether it did
func setUpdatedBy(db *gorm.DB, row any, userID *string) bool {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(row); err != nil {
		return false
	}

	field := stmt.Schema.LookUpField("UpdatedBy")
	if field == nil {
		return false
	}

	return field.Set(context.Background(), reflect.ValueOf(row).Elem(), userID) == nil
}
