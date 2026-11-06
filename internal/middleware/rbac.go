package middleware

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
)

// Authorize checks that the signed-in user holds every permission rules lists for the
// procedure, and returns the context with the user's permissions loaded. A procedure without
// a rule is rejected, so a new RPC stays closed until it has one.
func Authorize(ctx context.Context, db *database.DB, rules map[string][]string, procedure string) (context.Context, error) {
	required, ok := rules[procedure]
	if !ok {
		return nil, errors.PermissionDenied(errors.ErrPermissionDenied, fmt.Sprintf("no permission rule for %s", procedure))
	}

	ctx, err := WithPermissions(ctx, db)
	if err != nil {
		return nil, err
	}

	for _, permission := range required {
		if err := RequirePermission(ctx, db, permission); err != nil {
			return nil, err
		}
	}

	return ctx, nil
}

// NewRBACInterceptor runs Authorize before every unary call
func NewRBACInterceptor(db *database.DB, rules map[string][]string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ctx, err := Authorize(ctx, db, rules, req.Spec().Procedure)
			if err != nil {
				return nil, err
			}

			return next(ctx, req)
		}
	}
}
