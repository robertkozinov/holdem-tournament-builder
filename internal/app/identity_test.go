package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserID(t *testing.T) {
	t.Run("rejects absent identity", func(t *testing.T) {
		id, err := UserID(context.Background())
		require.ErrorIs(t, err, ErrUnauthenticated)
		assert.Equal(t, uuid.Nil, id)
	})
	t.Run("rejects nil identity", func(t *testing.T) {
		id, err := UserID(WithUserID(context.Background(), uuid.Nil))
		require.ErrorIs(t, err, ErrUnauthenticated)
		assert.Equal(t, uuid.Nil, id)
	})
	t.Run("preserves identity and parent cancellation", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		defer cancel()
		want := uuid.New()
		ctx := WithUserID(parent, want)
		id, err := UserID(ctx)
		require.NoError(t, err)
		assert.Equal(t, want, id)
		cancel()
		assert.ErrorIs(t, ctx.Err(), context.Canceled)
	})
}
