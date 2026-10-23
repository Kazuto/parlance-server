package common

import (
	"strings"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"gorm.io/gorm"
)

// Paginate counts the rows the query matches and loads the requested page of them into dest,
// preloading the named associations. It returns the pagination for the response.
func Paginate(query *gorm.DB, pagination *pb.PaginationRequest, dest any, preloads ...string) (*pb.PaginationResponse, error) {
	page, perPage := converter.GetPaginationParams(pagination)

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to count records")
	}

	find := query.Session(&gorm.Session{})
	for _, preload := range preloads {
		find = find.Preload(preload)
	}

	if err := find.Offset(int((page - 1) * perPage)).Limit(int(perPage)).Find(dest).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to fetch records")
	}

	return converter.PaginationToProto(total, page, perPage), nil
}

// SearchColumns returns a Filter search matching rows where any of the columns contains the
// search text, ignoring case
func SearchColumns(columns ...string) func(*gorm.DB, string) *gorm.DB {
	return func(query *gorm.DB, search string) *gorm.DB {
		pattern := "%" + EscapeLike(strings.ToLower(search)) + "%"
		conditions := make([]string, len(columns))
		args := make([]any, len(columns))

		for i, column := range columns {
			conditions[i] = "LOWER(" + column + `) LIKE ? ESCAPE '\'`
			args[i] = pattern
		}

		return query.Where(strings.Join(conditions, " OR "), args...)
	}
}

// EscapeLike escapes the LIKE wildcards in text, for patterns using ESCAPE '\'
func EscapeLike(text string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(text)
}
