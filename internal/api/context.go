package api

import (
	"context"

	"github.com/tdeshazo/home-api/internal/db"
)

type contextKey string

const userContextKey contextKey = "user"

func contextWithUser(ctx context.Context, user db.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func userFromContext(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(userContextKey).(db.User)
	return user, ok
}
