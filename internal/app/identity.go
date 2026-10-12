package app

import (
	"context"

	"github.com/google/uuid"
)

type userIDKey struct{}

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

func UserID(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	if !ok || id == uuid.Nil {
		return uuid.Nil, ErrUnauthenticated
	}
	return id, nil
}
