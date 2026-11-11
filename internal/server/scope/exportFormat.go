package scope

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
)

// exportFormat returns the file extension of a scope's default export format, or
// "" when it's unspecified.
func exportFormat(value pb.ExportFormat) (string, error) {
	if value == pb.ExportFormat_EXPORT_FORMAT_UNSPECIFIED {
		return "", nil
	}

	format := converter.ExportFormatFromProto(value)
	if format == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "unknown export format")
	}

	return format, nil
}
