package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statisticsRecord struct {
	playerID          uuid.UUID
	name              string
	tournamentsPlayed int64
	totalBuyIns       string
	totalPrizes       string
	netProfit         string
}

type statisticsRows struct {
	pgx.Rows
	records          []statisticsRecord
	index            int
	scanErr, readErr error
	closed           bool
}

func (rows *statisticsRows) Close() { rows.closed = true }

func (rows *statisticsRows) Err() error {
	if rows.closed {
		return rows.readErr
	}
	return nil
}

func (rows *statisticsRows) Next() bool {
	if rows.closed {
		return false
	}
	rows.index++
	if rows.index < len(rows.records) {
		return true
	}
	rows.Close()
	return false
}

func (rows *statisticsRows) Scan(dest ...any) error {
	if rows.scanErr != nil {
		rows.Close()
		return rows.scanErr
	}
	if rows.closed || rows.index < 0 || rows.index >= len(rows.records) {
		return errors.New("no current row")
	}
	if len(dest) != 6 {
		return fmt.Errorf("expected 6 scan destinations, got %d", len(dest))
	}
	record := rows.records[rows.index]
	*(dest[0].(*uuid.UUID)) = record.playerID
	*(dest[1].(*string)) = record.name
	*(dest[2].(*int64)) = record.tournamentsPlayed
	*(dest[3].(*string)) = record.totalBuyIns
	*(dest[4].(*string)) = record.totalPrizes
	*(dest[5].(*string)) = record.netProfit
	return nil
}

type statisticsDBStub struct {
	query func(context.Context, string, ...any) (pgx.Rows, error)
}

func (db *statisticsDBStub) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.query(ctx, query, args...)
}

func TestStatisticsRepository_MapsRowsAndForwardsPagination(t *testing.T) {
	owner := uuid.New()
	want := domain.PlayerStatistics{
		PlayerID:          uuid.New(),
		Name:              "Влад",
		TournamentsPlayed: 3,
		TotalBuyIns:       big.NewInt(4000),
		TotalPrizes:       big.NewInt(6500),
		NetProfit:         big.NewInt(2500),
	}
	rows := &statisticsRows{index: -1, records: []statisticsRecord{{
		playerID:          want.PlayerID,
		name:              want.Name,
		tournamentsPlayed: want.TournamentsPlayed,
		totalBuyIns:       want.TotalBuyIns.String(),
		totalPrizes:       want.TotalPrizes.String(),
		netProfit:         want.NetProfit.String(),
	}}}
	db := &statisticsDBStub{query: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
		assert.Equal(t, []any{owner, 25, 7}, args)
		return rows, nil
	}}

	got, err := (&StatisticsRepository{pool: db}).Leaderboard(context.Background(), owner, 25, 7)

	require.NoError(t, err)
	assert.Equal(t, []domain.PlayerStatistics{want}, got)
	assert.True(t, rows.closed)
}

func TestStatisticsRepository_ReturnsNonNilEmptySlice(t *testing.T) {
	rows := &statisticsRows{index: -1}
	db := &statisticsDBStub{query: func(context.Context, string, ...any) (pgx.Rows, error) {
		return rows, nil
	}}

	items, err := (&StatisticsRepository{pool: db}).Leaderboard(context.Background(), uuid.New(), 50, 0)

	require.NoError(t, err)
	assert.NotNil(t, items)
	assert.Empty(t, items)
}

func TestStatisticsRepository_RejectsInvalidAmounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record statisticsRecord
	}{
		{name: "total buy-ins", record: statisticsRecord{totalBuyIns: "invalid", totalPrizes: "0", netProfit: "0"}},
		{name: "total prizes", record: statisticsRecord{totalBuyIns: "0", totalPrizes: "1.5", netProfit: "0"}},
		{name: "net profit", record: statisticsRecord{totalBuyIns: "0", totalPrizes: "0", netProfit: ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := &statisticsRows{index: -1, records: []statisticsRecord{tc.record}}
			db := &statisticsDBStub{query: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			}}

			items, err := (&StatisticsRepository{pool: db}).Leaderboard(context.Background(), uuid.New(), 50, 0)

			require.ErrorContains(t, err, "invalid "+tc.name)
			assert.Nil(t, items)
			assert.True(t, rows.closed)
		})
	}
}

func TestStatisticsRepository_RejectsMissingOwner(t *testing.T) {
	items, err := NewStatisticsRepository(nil).Leaderboard(context.Background(), uuid.Nil, 50, 0)

	require.ErrorIs(t, err, app.ErrUnauthenticated)
	assert.Nil(t, items)
}

func TestStatisticsRepository_PreservesStorageErrors(t *testing.T) {
	queryErr := errors.New("query failed")
	db := &statisticsDBStub{query: func(context.Context, string, ...any) (pgx.Rows, error) {
		return nil, queryErr
	}}
	items, err := (&StatisticsRepository{pool: db}).Leaderboard(context.Background(), uuid.New(), 50, 0)
	require.ErrorIs(t, err, queryErr)
	assert.Nil(t, items)

	scanErr := errors.New("scan failed")
	rows := &statisticsRows{index: -1, records: []statisticsRecord{{}}, scanErr: scanErr}
	db.query = func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil }
	items, err = (&StatisticsRepository{pool: db}).Leaderboard(context.Background(), uuid.New(), 50, 0)
	require.ErrorIs(t, err, scanErr)
	assert.Nil(t, items)
	assert.True(t, rows.closed)

	readErr := errors.New("iteration failed")
	rows = &statisticsRows{index: -1, readErr: readErr}
	db.query = func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil }
	items, err = (&StatisticsRepository{pool: db}).Leaderboard(context.Background(), uuid.New(), 50, 0)
	require.ErrorIs(t, err, readErr)
	assert.Nil(t, items)
	assert.True(t, rows.closed)
}

type statisticsProjectionRow struct {
	playerID           uuid.UUID
	contribution       int64
	prize              int64
	includedStatistics bool
}

func setupStatisticsRepositoryTest(t *testing.T) (context.Context, *pgxpool.Pool, *StatisticsRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx, pool, _, owner := setupPlayerRepositoryTest(t)
	foreignOwner := uuid.New()
	require.NoError(t, NewUserRepository(pool).Create(ctx, domain.User{
		ID:           foreignOwner,
		Login:        "stats_" + foreignOwner.String()[:8],
		PasswordHash: "test-hash",
		CreatedAt:    time.Now().UTC(),
	}))
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, "DELETE FROM tournaments WHERE user_id = $1", foreignOwner)
		assert.NoError(t, err)
		_, err = pool.Exec(cleanupCtx, "DELETE FROM players WHERE user_id = $1", foreignOwner)
		assert.NoError(t, err)
		_, err = pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", foreignOwner)
		assert.NoError(t, err)
	})
	return ctx, pool, NewStatisticsRepository(pool), owner, foreignOwner
}

func insertStatisticsPlayer(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner, id uuid.UUID, name, normalized string, deleted bool) {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	_, err := pool.Exec(ctx, `INSERT INTO players
		(id, user_id, name, normalized_name, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW(), $5)`, id, owner, name, normalized, deletedAt)
	require.NoError(t, err)
}

func insertStatisticsTournament(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner uuid.UUID, rows ...statisticsProjectionRow) {
	t.Helper()
	tournamentID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO tournaments (id, name, tournament_date, data, user_id)
		VALUES ($1, 'Statistics Test', NOW(), '{}'::jsonb, $2)`, tournamentID, owner)
	require.NoError(t, err)
	for _, row := range rows {
		_, err = pool.Exec(ctx, `INSERT INTO tournament_players
			(tournament_id, player_id, user_id, contribution, prize, included_in_statistics)
			VALUES ($1, $2, $3, $4, $5, $6)`, tournamentID, row.playerID, owner, row.contribution, row.prize, row.includedStatistics)
		require.NoError(t, err)
	}
}

func TestStatisticsRepository_LeaderboardIntegration(t *testing.T) {
	ctx, pool, repo, owner, foreignOwner := setupStatisticsRepositoryTest(t)
	renamedID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	recreatedID := uuid.MustParse("00000000-0000-0000-0000-000000000102")
	alphaID := uuid.MustParse("00000000-0000-0000-0000-000000000104")
	betaID := uuid.MustParse("00000000-0000-0000-0000-000000000103")
	deletedID := uuid.MustParse("00000000-0000-0000-0000-000000000105")
	oldSameNameID := uuid.MustParse("00000000-0000-0000-0000-000000000106")
	draftOnlyID := uuid.MustParse("00000000-0000-0000-0000-000000000107")
	foreignID := uuid.MustParse("00000000-0000-0000-0000-000000000108")
	higherPrizeID := uuid.MustParse("00000000-0000-0000-0000-000000000109")
	registryOnlyID := uuid.MustParse("00000000-0000-0000-0000-000000000110")

	insertStatisticsPlayer(t, ctx, pool, owner, renamedID, "Before Rename", "before rename", false)
	insertStatisticsPlayer(t, ctx, pool, owner, recreatedID, "Return", "return", false)
	insertStatisticsPlayer(t, ctx, pool, owner, alphaID, "Alpha", "alpha", false)
	insertStatisticsPlayer(t, ctx, pool, owner, betaID, "Beta", "beta", false)
	insertStatisticsPlayer(t, ctx, pool, owner, deletedID, "Deleted", "deleted", true)
	insertStatisticsPlayer(t, ctx, pool, owner, oldSameNameID, "Return", "return", true)
	insertStatisticsPlayer(t, ctx, pool, owner, draftOnlyID, "Draft Only", "draft only", false)
	insertStatisticsPlayer(t, ctx, pool, foreignOwner, foreignID, "Foreign", "foreign", false)
	insertStatisticsPlayer(t, ctx, pool, owner, higherPrizeID, "Zulu", "zulu", false)
	insertStatisticsPlayer(t, ctx, pool, owner, registryOnlyID, "Registry Only", "registry only", false)

	insertStatisticsTournament(t, ctx, pool, owner,
		statisticsProjectionRow{playerID: renamedID, contribution: 1000, prize: 4000, includedStatistics: true},
		statisticsProjectionRow{playerID: alphaID, contribution: 1000, prize: 1000, includedStatistics: true},
		statisticsProjectionRow{playerID: deletedID, contribution: 100, prize: 10000, includedStatistics: true},
		statisticsProjectionRow{playerID: oldSameNameID, contribution: 100, prize: 8000, includedStatistics: true},
	)
	insertStatisticsTournament(t, ctx, pool, owner,
		statisticsProjectionRow{playerID: renamedID, contribution: 2000, prize: 0, includedStatistics: true},
		statisticsProjectionRow{playerID: recreatedID, contribution: 500, prize: 700, includedStatistics: true},
		statisticsProjectionRow{playerID: betaID, contribution: 1000, prize: 1000, includedStatistics: true},
		statisticsProjectionRow{playerID: higherPrizeID, contribution: 2000, prize: 2000, includedStatistics: true},
	)
	insertStatisticsTournament(t, ctx, pool, owner,
		statisticsProjectionRow{playerID: renamedID, contribution: 9000, prize: 9000, includedStatistics: false},
		statisticsProjectionRow{playerID: draftOnlyID, contribution: 1000, prize: 1000, includedStatistics: false},
	)
	insertStatisticsTournament(t, ctx, pool, foreignOwner,
		statisticsProjectionRow{playerID: foreignID, contribution: 1, prize: 99999, includedStatistics: true},
	)

	_, err := NewPlayerRepository(pool).Rename(ctx, owner, renamedID, "Renamed Player", "renamed player", time.Now().UTC())
	require.NoError(t, err)

	want := []domain.PlayerStatistics{
		{PlayerID: renamedID, Name: "Renamed Player", TournamentsPlayed: 2, TotalBuyIns: big.NewInt(3000), TotalPrizes: big.NewInt(4000), NetProfit: big.NewInt(1000)},
		{PlayerID: recreatedID, Name: "Return", TournamentsPlayed: 1, TotalBuyIns: big.NewInt(500), TotalPrizes: big.NewInt(700), NetProfit: big.NewInt(200)},
		{PlayerID: higherPrizeID, Name: "Zulu", TournamentsPlayed: 1, TotalBuyIns: big.NewInt(2000), TotalPrizes: big.NewInt(2000), NetProfit: big.NewInt(0)},
		{PlayerID: alphaID, Name: "Alpha", TournamentsPlayed: 1, TotalBuyIns: big.NewInt(1000), TotalPrizes: big.NewInt(1000), NetProfit: big.NewInt(0)},
		{PlayerID: betaID, Name: "Beta", TournamentsPlayed: 1, TotalBuyIns: big.NewInt(1000), TotalPrizes: big.NewInt(1000), NetProfit: big.NewInt(0)},
	}
	items, err := repo.Leaderboard(ctx, owner, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, want, items)

	page, err := repo.Leaderboard(ctx, owner, 2, 1)
	require.NoError(t, err)
	assert.Equal(t, want[1:3], page)

	foreignItems, err := repo.Leaderboard(ctx, foreignOwner, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, []domain.PlayerStatistics{{
		PlayerID:          foreignID,
		Name:              "Foreign",
		TournamentsPlayed: 1,
		TotalBuyIns:       big.NewInt(1),
		TotalPrizes:       big.NewInt(99999),
		NetProfit:         big.NewInt(99998),
	}}, foreignItems)

	empty, err := repo.Leaderboard(ctx, uuid.New(), 100, 0)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestStatisticsRepository_LeaderboardOverflowIntegration(t *testing.T) {
	ctx, pool, repo, owner, _ := setupStatisticsRepositoryTest(t)
	winnerID, loserID := uuid.New(), uuid.New()
	higherPrizeID, lowerPrizeID := uuid.New(), uuid.New()
	insertStatisticsPlayer(t, ctx, pool, owner, winnerID, "Winner", "winner", false)
	insertStatisticsPlayer(t, ctx, pool, owner, loserID, "Loser", "loser", false)
	insertStatisticsPlayer(t, ctx, pool, owner, higherPrizeID, "Zulu", "zulu", false)
	insertStatisticsPlayer(t, ctx, pool, owner, lowerPrizeID, "Alpha", "alpha", false)
	for range 4 {
		insertStatisticsTournament(t, ctx, pool, owner,
			statisticsProjectionRow{playerID: winnerID, contribution: 3000000000000000000, prize: 6000000000000000000, includedStatistics: true},
			statisticsProjectionRow{playerID: loserID, contribution: 3000000000000000000, includedStatistics: true},
			statisticsProjectionRow{playerID: higherPrizeID, contribution: 3000000000000000000, prize: 3000000000000000000, includedStatistics: true},
		)
	}
	for range 3 {
		insertStatisticsTournament(t, ctx, pool, owner,
			statisticsProjectionRow{playerID: lowerPrizeID, contribution: 3000000000000000000, prize: 3000000000000000000, includedStatistics: true},
		)
	}

	items, err := repo.Leaderboard(ctx, owner, 100, 0)

	require.NoError(t, err)
	require.Len(t, items, 4)
	assert.Equal(t, winnerID, items[0].PlayerID)
	assert.Equal(t, int64(4), items[0].TournamentsPlayed)
	assert.Equal(t, "12000000000000000000", items[0].TotalBuyIns.String())
	assert.Equal(t, "24000000000000000000", items[0].TotalPrizes.String())
	assert.Equal(t, "12000000000000000000", items[0].NetProfit.String())
	assert.Equal(t, higherPrizeID, items[1].PlayerID)
	assert.Equal(t, "12000000000000000000", items[1].TotalPrizes.String())
	assert.Equal(t, "0", items[1].NetProfit.String())
	assert.Equal(t, lowerPrizeID, items[2].PlayerID)
	assert.Equal(t, "9000000000000000000", items[2].TotalPrizes.String())
	assert.Equal(t, "0", items[2].NetProfit.String())
	assert.Equal(t, loserID, items[3].PlayerID)
	assert.Equal(t, "12000000000000000000", items[3].TotalBuyIns.String())
	assert.Equal(t, "0", items[3].TotalPrizes.String())
	assert.Equal(t, "-12000000000000000000", items[3].NetProfit.String())
}
