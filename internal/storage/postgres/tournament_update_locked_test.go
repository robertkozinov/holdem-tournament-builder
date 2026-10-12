package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentRepository_UpdateLocked(t *testing.T) {
	tr := validTournament(t)
	id := uuid.New()
	// The SQL row identity wins over a mismatched identity inside JSON.
	tr.ID = uuid.New()
	data, err := json.Marshal(tr)
	require.NoError(t, err)
	var events []string
	var changed *domain.Tournament
	tx := &stubTournamentTx{
		queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
			events = append(events, "select")
			assert.Equal(t, "SELECT data FROM tournaments WHERE id = $1 AND user_id = $2 FOR UPDATE", compactSQL(query))
			assert.Equal(t, []any{id, testOwnerID}, args)
			return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*[]byte)) = data; return nil })
		},
		exec: func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
			events = append(events, "update")
			assert.Equal(t, "UPDATE tournaments SET name = $1, tournament_date = $2, data = $3, updated_at = NOW() WHERE id = $4 AND user_id = $5", compactSQL(query))
			require.Len(t, args, 5)
			assert.Equal(t, []any{changed.Name, changed.Date}, args[:2])
			assertStoredTournament(t, changed, args[2].([]byte))
			assert.Equal(t, id, args[3])
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		commit: func(context.Context) error { events = append(events, "commit"); return nil },
		rollback: func(ctx context.Context) error {
			events = append(events, "rollback")
			assert.NoError(t, ctx.Err())
			_, ok := ctx.Deadline()
			assert.True(t, ok, "rollback cleanup must be bounded")
			return pgx.ErrTxClosed
		},
	}
	db := &stubTournamentDB{beginTx: func(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
		events = append(events, "begin")
		assert.Equal(t, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, options)
		return tx, nil
	}}
	got, err := (&TournamentRepository{pool: db}).UpdateLocked(context.Background(), testOwnerID, id, func(current *domain.Tournament) (*domain.Tournament, error) {
		events = append(events, "change")
		expected := *tr
		expected.ID = id
		assert.Equal(t, &expected, current)
		current.Name = "Updated Game"
		current.Players[0].BuyInPaid = true
		changed = current
		return current, nil
	})
	require.NoError(t, err)
	assert.Same(t, changed, got)
	assert.Equal(t, []string{"begin", "select", "change", "update", "commit", "rollback"}, events)
}

func TestTournamentRepository_UpdateLockedErrors(t *testing.T) {
	cause := errors.New("transaction failure")
	for _, tc := range []struct {
		stage      string
		wantErr    error
		wantEvents []string
	}{
		{"begin", cause, []string{"begin"}},
		{"read", cause, []string{"begin", "select", "rollback"}},
		{"not found", app.ErrTournamentNotFound, []string{"begin", "select", "rollback"}},
		{"invalid json", nil, []string{"begin", "select", "rollback"}},
		{"change", cause, []string{"begin", "select", "change", "rollback"}},
		{"nil candidate", errNilTournament, []string{"begin", "select", "change", "rollback"}},
		{"changed id", nil, []string{"begin", "select", "change", "rollback"}},
		{"marshal", nil, []string{"begin", "select", "change", "rollback"}},
		{"write", cause, []string{"begin", "select", "change", "update", "rollback"}},
		{"missing on write", app.ErrTournamentNotFound, []string{"begin", "select", "change", "update", "rollback"}},
		{"commit", cause, []string{"begin", "select", "change", "update", "commit", "rollback"}},
		{"commit rolled back", pgx.ErrTxCommitRollback, []string{"begin", "select", "change", "update", "commit", "rollback"}},
		{"cancelled request", context.Canceled, []string{"begin", "select", "change", "rollback"}},
		{"cancelled write", context.Canceled, []string{"begin", "select", "change", "update", "rollback"}},
		{"cancelled commit", context.Canceled, []string{"begin", "select", "change", "update", "commit", "rollback"}},
		{"rollback failure", cause, []string{"begin", "select", "change", "rollback"}},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tr := validTournament(t)
			tr.ID = uuid.New()
			data, err := json.Marshal(tr)
			require.NoError(t, err)
			var events []string
			tx := &stubTournamentTx{
				queryRow: func(context.Context, string, ...any) pgx.Row {
					events = append(events, "select")
					return tournamentRowFunc(func(dest ...any) error {
						switch tc.stage {
						case "read":
							return cause
						case "not found":
							return pgx.ErrNoRows
						case "invalid json":
							*(dest[0].(*[]byte)) = []byte(`{"Players":"bad"}`)
						default:
							*(dest[0].(*[]byte)) = data
						}
						return nil
					})
				},
				exec: func(execCtx context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					events = append(events, "update")
					if tc.stage == "cancelled write" {
						return pgconn.CommandTag{}, execCtx.Err()
					}
					if tc.stage == "cancelled commit" {
						cancel()
					}
					if tc.stage == "write" {
						return pgconn.CommandTag{}, cause
					}
					if tc.stage == "missing on write" {
						return pgconn.NewCommandTag("UPDATE 0"), nil
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
				commit: func(commitCtx context.Context) error {
					events = append(events, "commit")
					if tc.stage == "cancelled commit" {
						return commitCtx.Err()
					}
					if tc.stage == "commit rolled back" {
						return pgx.ErrTxCommitRollback
					}
					return cause
				},
				rollback: func(cleanupCtx context.Context) error {
					events = append(events, "rollback")
					assert.NoError(t, cleanupCtx.Err())
					deadline, ok := cleanupCtx.Deadline()
					assert.True(t, ok)
					assert.LessOrEqual(t, time.Until(deadline), 5*time.Second)
					if tc.stage == "rollback failure" {
						return errors.New("rollback failed")
					}
					return nil
				},
			}
			db := &stubTournamentDB{beginTx: func(context.Context, pgx.TxOptions) (pgx.Tx, error) {
				events = append(events, "begin")
				if tc.stage == "begin" {
					return nil, cause
				}
				return tx, nil
			}}
			got, err := (&TournamentRepository{pool: db}).UpdateLocked(ctx, testOwnerID, tr.ID, func(current *domain.Tournament) (*domain.Tournament, error) {
				events = append(events, "change")
				switch tc.stage {
				case "change", "rollback failure":
					return nil, cause
				case "nil candidate":
					return nil, nil
				case "changed id":
					current.ID = uuid.New()
				case "marshal":
					current.Date = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
				case "cancelled request":
					cancel()
					return nil, ctx.Err()
				case "cancelled write":
					cancel()
				}
				return current, nil
			})
			require.Error(t, err)
			assert.Nil(t, got)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			}
			if tc.stage == "change" || tc.stage == "rollback failure" {
				assert.Same(t, cause, err, "the repository must preserve the service callback error")
			}
			if tc.stage == "invalid json" {
				var jsonErr *json.UnmarshalTypeError
				assert.ErrorAs(t, err, &jsonErr)
			}
			if tc.stage == "changed id" {
				assert.Contains(t, err.Error(), "id does not match locked row")
			}
			assert.Equal(t, tc.wantEvents, events)
		})
	}
	t.Run("invalid id does not begin transaction", func(t *testing.T) {
		called := false
		got, err := NewTournamentRepository(nil).UpdateLocked(context.Background(), testOwnerID, uuid.Nil, func(tr *domain.Tournament) (*domain.Tournament, error) {
			called = true
			return tr, nil
		})
		require.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, got)
		assert.False(t, called)
	})
}

type stubTournamentTx struct {
	pgx.Tx
	queryRow func(context.Context, string, ...any) pgx.Row
	exec     func(context.Context, string, ...any) (pgconn.CommandTag, error)
	commit   func(context.Context) error
	rollback func(context.Context) error
}

func (tx *stubTournamentTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return tx.queryRow(ctx, query, args...)
}

func (tx *stubTournamentTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return tx.exec(ctx, query, args...)
}

func (tx *stubTournamentTx) Commit(ctx context.Context) error   { return tx.commit(ctx) }
func (tx *stubTournamentTx) Rollback(ctx context.Context) error { return tx.rollback(ctx) }
