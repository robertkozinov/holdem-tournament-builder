package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTournamentRepositoryTest(t *testing.T) (context.Context, *pgxpool.Pool, *TournamentRepository) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("integration test skipped: TEST_DATABASE_URL is not set")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(config.ConnConfig.Database, "_test"), "refusing to connect to a non-test database")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	require.NoError(t, verifyTournamentTestDatabase(ctx, config.ConnConfig.Database, conn))
	_, err = conn.Exec(ctx, "TRUNCATE TABLE tournaments")
	require.NoError(t, err)
	return ctx, pool, NewTournamentRepository(pool)
}

func verifyTournamentTestDatabase(ctx context.Context, expected string, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	if !strings.HasSuffix(expected, "_test") {
		return fmt.Errorf("refusing to clear database %q: name must end in _test", expected)
	}
	var actual string
	if err := db.QueryRow(ctx, "SELECT current_database()").Scan(&actual); err != nil {
		return fmt.Errorf("verify test database: %w", err)
	}
	if actual != expected || !strings.HasSuffix(actual, "_test") {
		return fmt.Errorf("refusing to clear database %q: expected %q", actual, expected)
	}
	return nil
}

func validTournament(t *testing.T) *domain.Tournament {
	t.Helper()
	firstPlace := 1
	players := []domain.TournamentPlayer{
		{ID: uuid.New(), Name: "A", BuyInPaid: false, Rebuys: []domain.Rebuy{{ID: uuid.New(), Paid: true}, {ID: uuid.New(), Paid: false}}},
		{ID: uuid.New(), Name: "B", BuyInPaid: true, Rebuys: []domain.Rebuy{{ID: uuid.New(), Paid: false}}},
	}
	return &domain.Tournament{
		Name: "Friday Game", Date: time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC),
		Players: players, BuyInAmount: 1000, SettlementMode: domain.SettlementP2P,
		ChipSet: []domain.ChipDenomination{{Value: 25, Count: 100}, {Value: 100, Count: 100}},
		Style:   domain.StyleTurbo, Duration: 2 * time.Hour, LevelDuration: 10 * time.Minute,
		StartingStack: domain.StackPlan{Distribution: []domain.ChipDenomination{{Value: 25, Count: 10}, {Value: 100, Count: 10}}},
		BlindStructure: []domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Duration: 10 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Ante: 100, Duration: 10 * time.Minute},
		},
		Results: []domain.Result{
			{PlayerID: players[0].ID, Place: &firstPlace, Prize: 3000},
			{PlayerID: players[1].ID, Place: nil, Prize: 2000},
		},
	}
}

func TestTournamentRepository_Create(t *testing.T) {
	t.Run("creates tournament", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id)
		assert.Equal(t, id, tr.ID)
		var name string
		var storedID uuid.UUID
		var storedDate time.Time
		err = pool.QueryRow(ctx, "SELECT id, name, tournament_date FROM tournaments WHERE id = $1", id).Scan(&storedID, &name, &storedDate)
		require.NoError(t, err)
		assert.Equal(t, id, storedID)
		assert.Equal(t, tr.Name, name)
		assert.True(t, tr.Date.Equal(storedDate))
	})
	t.Run("returns error when tournament is nil", func(t *testing.T) {
		id, err := NewTournamentRepository(nil).Create(context.Background(), nil)
		require.ErrorIs(t, err, errNilTournament)
		assert.Equal(t, uuid.Nil, id)
	})
	t.Run("returns error when id already exists", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		first := validTournament(t)
		first.ID = uuid.New()
		createdID, err := repo.Create(ctx, first)
		require.NoError(t, err)
		assert.Equal(t, first.ID, createdID)
		second := validTournament(t)
		second.ID = first.ID
		id, err := repo.Create(ctx, second)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, "23505", pgErr.Code)
		assert.Equal(t, uuid.Nil, id)
	})
	t.Run("stores complete card without status", func(t *testing.T) {
		tr := validTournament(t)
		db := &stubTournamentDB{exec: func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
			assert.Equal(t, "INSERT INTO tournaments (id, name, tournament_date, data) VALUES ($1, $2, $3, $4)", compactSQL(query))
			require.Len(t, args, 4)
			assert.Equal(t, []any{tr.ID, tr.Name, tr.Date}, args[:3])
			assertStoredTournament(t, tr, args[3].([]byte))
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		}}
		id, err := (&TournamentRepository{pool: db}).Create(context.Background(), tr)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id)
		assert.Equal(t, tr.ID, id)
	})
}

func TestTournamentRepository_GetByID(t *testing.T) {
	t.Run("gets complete tournament", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		got, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, tr, got)
	})
	t.Run("returns error when id is empty", func(t *testing.T) {
		got, err := NewTournamentRepository(nil).GetByID(context.Background(), uuid.Nil)
		require.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, got)
	})
	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		got, err := repo.GetByID(ctx, uuid.New())
		require.ErrorIs(t, err, app.ErrTournamentNotFound)
		assert.Nil(t, got)
	})
	t.Run("uses row id instead of json id", func(t *testing.T) {
		tr := validTournament(t)
		tr.ID = uuid.New()
		data, err := json.Marshal(tr)
		require.NoError(t, err)
		id := uuid.New()
		db := &stubTournamentDB{queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
			assert.Equal(t, "SELECT data FROM tournaments WHERE id = $1", compactSQL(query))
			assert.Equal(t, []any{id}, args)
			return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*[]byte)) = data; return nil })
		}}
		got, err := (&TournamentRepository{pool: db}).GetByID(context.Background(), id)
		require.NoError(t, err)
		tr.ID = id
		assert.Equal(t, tr, got)
	})
	t.Run("returns error for invalid tournament json", func(t *testing.T) {
		db := &stubTournamentDB{queryRow: func(context.Context, string, ...any) pgx.Row {
			return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*[]byte)) = []byte(`{"Players":"invalid"}`); return nil })
		}}
		got, err := (&TournamentRepository{pool: db}).GetByID(context.Background(), uuid.New())
		var cause *json.UnmarshalTypeError
		require.ErrorAs(t, err, &cause)
		assert.Nil(t, got)
	})
}

func TestTournamentRepository_Update(t *testing.T) {
	t.Run("replaces complete tournament card", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		tr.Name = "Saturday Game"
		tr.Date = tr.Date.AddDate(0, 0, 1)
		tr.SettlementMode = domain.SettlementBank
		tr.Style = domain.StyleDeep
		tr.Duration = 3 * time.Hour
		tr.LevelDuration = 15 * time.Minute
		tr.ChipSet = []domain.ChipDenomination{{Value: 50, Count: 200}}
		tr.StartingStack = domain.StackPlan{Distribution: []domain.ChipDenomination{{Value: 50, Count: 20}}}
		tr.BlindStructure = []domain.BlindLevel{{SmallBlind: 50, BigBlind: 100, Duration: 15 * time.Minute}}
		tr.Players[0].Name = "Renamed A"
		tr.Players[0].BuyInPaid = true
		tr.Players[0].Rebuys[1].Paid = true
		tr.Players[1].Rebuys = []domain.Rebuy{}
		tr.Players = append(tr.Players, domain.TournamentPlayer{ID: uuid.New(), Name: "C", Rebuys: []domain.Rebuy{}})
		tr.Results = []domain.Result{{PlayerID: tr.Players[1].ID, Place: nil, Prize: 5000}}
		for i := 0; i < 2; i++ {
			require.NoError(t, repo.Update(ctx, tr))
			got, err := repo.GetByID(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, tr, got)
		}
		var storedName string
		var storedDate time.Time
		var count int
		err = pool.QueryRow(ctx, "SELECT name, tournament_date FROM tournaments WHERE id = $1", id).Scan(&storedName, &storedDate)
		require.NoError(t, err)
		assert.Equal(t, tr.Name, storedName)
		assert.True(t, tr.Date.Equal(storedDate))
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM tournaments").Scan(&count))
		assert.Equal(t, 1, count)
		tr.Results = []domain.Result{}
		require.NoError(t, repo.Update(ctx, tr))
		got, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, tr, got)
		assert.NotNil(t, got.Results)
		assert.Empty(t, got.Results)
	})
	t.Run("returns error when tournament is nil", func(t *testing.T) {
		require.ErrorIs(t, NewTournamentRepository(nil).Update(context.Background(), nil), errNilTournament)
	})
	t.Run("returns error when id is empty", func(t *testing.T) {
		require.ErrorIs(t, NewTournamentRepository(nil).Update(context.Background(), validTournament(t)), app.ErrInvalidTournamentID)
	})
	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		tr.ID = uuid.New()
		require.ErrorIs(t, repo.Update(ctx, tr), app.ErrTournamentNotFound)
	})
	t.Run("stores complete card without status", func(t *testing.T) {
		tr := validTournament(t)
		tr.ID = uuid.New()
		db := &stubTournamentDB{exec: func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
			assert.Equal(t, "UPDATE tournaments SET name = $1, tournament_date = $2, data = $3, updated_at = NOW() WHERE id = $4", compactSQL(query))
			require.Len(t, args, 4)
			assert.Equal(t, []any{tr.Name, tr.Date}, args[:2])
			assertStoredTournament(t, tr, args[2].([]byte))
			assert.Equal(t, tr.ID, args[3])
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}}
		require.NoError(t, (&TournamentRepository{pool: db}).Update(context.Background(), tr))
	})
}

func TestTournamentRepository_Delete(t *testing.T) {
	t.Run("deletes tournament", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		id, err := repo.Create(ctx, validTournament(t))
		require.NoError(t, err)
		require.NoError(t, repo.Delete(ctx, id))
		got, err := repo.GetByID(ctx, id)
		require.ErrorIs(t, err, app.ErrTournamentNotFound)
		assert.Nil(t, got)
	})
	t.Run("returns error when id is empty", func(t *testing.T) {
		require.ErrorIs(t, NewTournamentRepository(nil).Delete(context.Background(), uuid.Nil), app.ErrInvalidTournamentID)
	})
	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		require.ErrorIs(t, repo.Delete(ctx, uuid.New()), app.ErrTournamentNotFound)
	})
}

func TestTournamentRepository_MissingRows(t *testing.T) {
	db := &stubTournamentDB{
		exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return tournamentRowFunc(func(...any) error { return pgx.ErrNoRows })
		},
	}
	repo := &TournamentRepository{pool: db}
	tr := validTournament(t)
	tr.ID = uuid.New()
	got, err := repo.GetByID(context.Background(), tr.ID)
	require.ErrorIs(t, err, app.ErrTournamentNotFound)
	assert.Nil(t, got)
	require.ErrorIs(t, repo.Update(context.Background(), tr), app.ErrTournamentNotFound)
	require.ErrorIs(t, repo.Delete(context.Background(), tr.ID), app.ErrTournamentNotFound)
}

func TestTournamentRepository_WrapsDatabaseErrors(t *testing.T) {
	cause := errors.New("database failed")
	db := &stubTournamentDB{
		exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, cause },
		queryRow: func(context.Context, string, ...any) pgx.Row {
			return tournamentRowFunc(func(...any) error { return cause })
		},
	}
	repo := &TournamentRepository{pool: db}
	tr := validTournament(t)
	tr.ID = uuid.New()
	id, err := repo.Create(context.Background(), tr)
	require.ErrorIs(t, err, cause)
	assert.Equal(t, uuid.Nil, id)
	got, err := repo.GetByID(context.Background(), tr.ID)
	require.ErrorIs(t, err, cause)
	assert.Nil(t, got)
	require.ErrorIs(t, repo.Update(context.Background(), tr), cause)
	require.ErrorIs(t, repo.Delete(context.Background(), tr.ID), cause)
}

func TestTournamentRepository_ListQueryError(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://unused@127.0.0.1:1/holdem_test?sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tournaments, err := NewTournamentRepository(pool).List(ctx, 50, 0)
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, tournaments)
}

func TestTournamentRepository_List(t *testing.T) {
	t.Run("returns empty nonnil slice", func(t *testing.T) {
		rows := &stubTournamentRows{index: -1}
		db := &stubTournamentDB{query: func(_ context.Context, query string, args ...any) (pgx.Rows, error) {
			assert.Equal(t, "SELECT id, data FROM tournaments ORDER BY tournament_date DESC, id DESC LIMIT $1 OFFSET $2", compactSQL(query))
			assert.Equal(t, []any{50, 5}, args)
			return rows, nil
		}}
		got, err := (&TournamentRepository{pool: db}).List(context.Background(), 50, 5)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
		assert.True(t, rows.closed)
	})
	t.Run("decodes cards and uses row ids", func(t *testing.T) {
		first, second := validTournament(t), validTournament(t)
		first.ID, second.ID = uuid.New(), uuid.New()
		firstData, err := json.Marshal(first)
		require.NoError(t, err)
		secondData, err := json.Marshal(second)
		require.NoError(t, err)
		ids := []uuid.UUID{uuid.New(), uuid.New()}
		rows := &stubTournamentRows{index: -1, records: []storedTournamentRow{{ids[0], firstData}, {ids[1], secondData}}}
		db := &stubTournamentDB{query: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil }}
		got, err := (&TournamentRepository{pool: db}).List(context.Background(), 2, 0)
		require.NoError(t, err)
		first.ID, second.ID = ids[0], ids[1]
		assert.Equal(t, []*domain.Tournament{first, second}, got)
		assert.True(t, rows.closed)
	})
	t.Run("sorts by date and id with pagination", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		cards := make([]*domain.Tournament, 4)
		for i := range cards {
			cards[i] = validTournament(t)
			cards[i].ID = uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1))
		}
		cards[1].Date = cards[1].Date.AddDate(0, 0, 1)
		cards[2].Date = cards[1].Date
		cards[3].Date = cards[3].Date.AddDate(0, 0, -1)
		for _, index := range []int{1, 3, 0, 2} {
			_, err := repo.Create(ctx, cards[index])
			require.NoError(t, err)
		}
		ordered := []*domain.Tournament{cards[2], cards[1], cards[0], cards[3]}
		for _, page := range []struct{ limit, offset int }{{100, 0}, {2, 0}, {2, 1}, {2, 2}, {1, 3}, {2, 4}, {1, 100}} {
			got, err := repo.List(ctx, page.limit, page.offset)
			require.NoError(t, err)
			require.NotNil(t, got)
			if page.offset >= len(ordered) {
				assert.Empty(t, got)
				continue
			}
			end := min(page.offset+page.limit, len(ordered))
			assert.Equal(t, ordered[page.offset:end], got)
		}
		cards[3].Date = cards[1].Date.AddDate(0, 0, 1)
		require.NoError(t, repo.Update(ctx, cards[3]))
		got, err := repo.List(ctx, 1, 0)
		require.NoError(t, err)
		assert.Equal(t, []*domain.Tournament{cards[3]}, got)
	})
	t.Run("wraps query error", func(t *testing.T) {
		cause := errors.New("query failed")
		db := &stubTournamentDB{query: func(context.Context, string, ...any) (pgx.Rows, error) { return nil, cause }}
		got, err := (&TournamentRepository{pool: db}).List(context.Background(), 50, 0)
		require.ErrorIs(t, err, cause)
		assert.Nil(t, got)
	})
	for _, name := range []string{"scan error", "row error", "invalid json"} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(validTournament(t))
			require.NoError(t, err)
			rows := &stubTournamentRows{index: -1, records: []storedTournamentRow{{uuid.New(), data}}}
			cause := errors.New(name)
			switch name {
			case "scan error":
				rows.scanErr = cause
			case "row error":
				rows.readErr = cause
			case "invalid json":
				rows.records[0].data = []byte(`{"Results":"invalid"}`)
			}
			db := &stubTournamentDB{query: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil }}
			got, err := (&TournamentRepository{pool: db}).List(context.Background(), 50, 0)
			if name == "invalid json" {
				var jsonErr *json.UnmarshalTypeError
				require.ErrorAs(t, err, &jsonErr)
			} else {
				require.ErrorIs(t, err, cause)
			}
			assert.Nil(t, got)
			assert.True(t, rows.closed)
		})
	}
}

func TestVerifyTournamentTestDatabase(t *testing.T) {
	for _, tc := range []struct {
		name, expected, actual string
		wantError, wantQuery   bool
		scanError              error
	}{
		{name: "verified test database", expected: "holdem_test", actual: "holdem_test", wantQuery: true},
		{name: "configured working database", expected: "holdem", actual: "holdem", wantError: true},
		{name: "test is not a suffix", expected: "holdem_test_backup", wantError: true},
		{name: "empty configured database", wantError: true},
		{name: "actual working database", expected: "holdem_test", actual: "holdem", wantError: true, wantQuery: true},
		{name: "different test database", expected: "holdem_test", actual: "other_test", wantError: true, wantQuery: true},
		{name: "verification error", expected: "holdem_test", wantError: true, wantQuery: true, scanError: errors.New("verification failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queried := false
			db := &stubTournamentDB{queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
				queried = true
				assert.Equal(t, "SELECT current_database()", query)
				assert.Empty(t, args)
				return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*string)) = tc.actual; return tc.scanError })
			}}
			err := verifyTournamentTestDatabase(context.Background(), tc.expected, db)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.scanError != nil {
				require.ErrorIs(t, err, tc.scanError)
			}
			assert.Equal(t, tc.wantQuery, queried)
		})
	}
}

func TestTournamentStatusMigrations(t *testing.T) {
	for _, tc := range []struct{ suffix, statement string }{
		{"up", "ALTER TABLE tournaments DROP COLUMN status;"},
		{"down", "ALTER TABLE tournaments ADD COLUMN status TEXT NOT NULL DEFAULT 'created';"},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "migrations", "000002_drop_tournament_status."+tc.suffix+".sql")
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, compactSQL(string(data)), tc.statement)
		})
	}
}

func assertStoredTournament(t *testing.T, expected *domain.Tournament, data []byte) {
	t.Helper()
	var got domain.Tournament
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, expected, &got)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fields))
	for _, field := range []string{"Status", "PayoutSpots", "Contributions", "storedContributions"} {
		assert.NotContains(t, fields, field)
	}
}

func compactSQL(query string) string { return strings.Join(strings.Fields(query), " ") }

type stubTournamentDB struct {
	exec     func(context.Context, string, ...any) (pgconn.CommandTag, error)
	query    func(context.Context, string, ...any) (pgx.Rows, error)
	queryRow func(context.Context, string, ...any) pgx.Row
	beginTx  func(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

func (db *stubTournamentDB) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return db.exec(ctx, query, args...)
}
func (db *stubTournamentDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.query(ctx, query, args...)
}
func (db *stubTournamentDB) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return db.queryRow(ctx, query, args...)
}
func (db *stubTournamentDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	return db.beginTx(ctx, options)
}

type tournamentRowFunc func(...any) error

func (row tournamentRowFunc) Scan(dest ...any) error { return row(dest...) }

type storedTournamentRow struct {
	id   uuid.UUID
	data []byte
}
type stubTournamentRows struct {
	pgx.Rows
	records          []storedTournamentRow
	index            int
	scanErr, readErr error
	closed           bool
}

func (rows *stubTournamentRows) Close() { rows.closed = true }
func (rows *stubTournamentRows) Err() error {
	if rows.closed {
		return rows.readErr
	}
	return nil
}
func (rows *stubTournamentRows) Next() bool {
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
func (rows *stubTournamentRows) Scan(dest ...any) error {
	if rows.scanErr != nil {
		rows.Close()
		return rows.scanErr
	}
	if rows.closed || rows.index < 0 || rows.index >= len(rows.records) {
		return errors.New("no current row")
	}
	if len(dest) != 2 {
		return fmt.Errorf("expected 2 scan destinations, got %d", len(dest))
	}
	*(dest[0].(*uuid.UUID)) = rows.records[rows.index].id
	*(dest[1].(*[]byte)) = rows.records[rows.index].data
	return nil
}
