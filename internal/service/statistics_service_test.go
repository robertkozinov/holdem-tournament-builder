package service

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statisticsRepoStub struct {
	owner         uuid.UUID
	limit, offset int
	items         []domain.PlayerStatistics
	called        bool
	err           error
}

func (r *statisticsRepoStub) Leaderboard(_ context.Context, owner uuid.UUID, limit, offset int) ([]domain.PlayerStatistics, error) {
	r.called, r.owner, r.limit, r.offset = true, owner, limit, offset
	return r.items, r.err
}

func TestStatisticsService_Leaderboard(t *testing.T) {
	owner := uuid.New()
	items := []domain.PlayerStatistics{{
		PlayerID:          uuid.New(),
		Name:              "Влад",
		TournamentsPlayed: 3,
		TotalBuyIns:       big.NewInt(4000),
		TotalPrizes:       big.NewInt(6500),
		NetProfit:         big.NewInt(2500),
	}}
	repo := &statisticsRepoStub{items: items}

	got, err := NewStatisticsService(repo).Leaderboard(app.WithUserID(context.Background(), owner), 25, 7)

	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, owner, repo.owner)
	assert.Equal(t, 25, repo.limit)
	assert.Equal(t, 7, repo.offset)
}

func TestStatisticsService_RejectsBeforeStorage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		limit  int
		offset int
		want   error
	}{
		{name: "missing identity", ctx: context.Background(), limit: 50, want: app.ErrUnauthenticated},
		{name: "zero limit", ctx: app.WithUserID(context.Background(), uuid.New()), limit: 0, want: app.ErrValidation},
		{name: "limit above maximum", ctx: app.WithUserID(context.Background(), uuid.New()), limit: 101, want: app.ErrValidation},
		{name: "negative offset", ctx: app.WithUserID(context.Background(), uuid.New()), limit: 50, offset: -1, want: app.ErrValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &statisticsRepoStub{}

			items, err := NewStatisticsService(repo).Leaderboard(tc.ctx, tc.limit, tc.offset)

			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, items)
			assert.False(t, repo.called)
		})
	}
}

func TestStatisticsService_PreservesRepositoryError(t *testing.T) {
	cause := errors.New("storage failed")
	repo := &statisticsRepoStub{err: cause}
	ctx := app.WithUserID(context.Background(), uuid.New())

	items, err := NewStatisticsService(repo).Leaderboard(ctx, 100, 0)

	require.Error(t, err)
	require.ErrorIs(t, err, cause)
	assert.EqualError(t, err, "list player statistics: storage failed")
	assert.Nil(t, items)
}

func TestStatisticsService_PreservesNonNilEmptySlice(t *testing.T) {
	repo := &statisticsRepoStub{items: make([]domain.PlayerStatistics, 0)}
	ctx := app.WithUserID(context.Background(), uuid.New())

	items, err := NewStatisticsService(repo).Leaderboard(ctx, 1, 0)

	require.NoError(t, err)
	assert.NotNil(t, items)
	assert.Empty(t, items)
}
