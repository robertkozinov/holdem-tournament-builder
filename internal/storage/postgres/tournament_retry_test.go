package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentSaveRetries(t *testing.T) {
	for _, tc := range []struct {
		name               string
		cause              error
		failures, attempts int
		cancel             bool
	}{
		{"deadlock then success", &pgconn.PgError{Code: "40P01"}, 1, 2, false},
		{"serialization then success", &pgconn.PgError{Code: "40001"}, 1, 2, false},
		{"name resolution then success", errPlayerResolutionChanged, 1, 2, false},
		{"three attempts maximum", &pgconn.PgError{Code: "40P01"}, 10, 3, false},
		{"cancel prevents retry", &pgconn.PgError{Code: "40001"}, 1, 1, true},
		{"other database error not retried", &pgconn.PgError{Code: "23503"}, 1, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			card := validTournament(t)
			var events []string
			attempts := 0
			db := &stubTournamentDB{beginTx: func(context.Context, pgx.TxOptions) (pgx.Tx, error) {
				attempts++
				attempt := attempts
				events = append(events, fmt.Sprintf("begin%d", attempt))
				return &stubTournamentTx{
					query: func(context.Context, string, ...any) (pgx.Rows, error) {
						return registryRows(stubRegistryPlayers(card)...), nil
					},
					exec: func(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
						if strings.Contains(query, "INSERT INTO tournaments") && attempt <= tc.failures {
							if tc.cancel {
								cancel()
							}
							return pgconn.CommandTag{}, tc.cause
						}
						return pgconn.NewCommandTag("INSERT 0 1"), nil
					},
					commit: func(context.Context) error { events = append(events, fmt.Sprintf("commit%d", attempt)); return nil },
					rollback: func(cleanup context.Context) error {
						assert.NoError(t, cleanup.Err())
						events = append(events, fmt.Sprintf("rollback%d", attempt))
						return nil
					},
				}, nil
			}}
			got, err := (&TournamentRepository{pool: db}).SaveWithPlayers(ctx, testOwnerID, uuid.Nil, card.Players, registryTournamentBuilder(card))
			assert.Equal(t, tc.attempts, attempts)
			var expected []string
			for attempt := 1; attempt <= tc.attempts; attempt++ {
				expected = append(expected, fmt.Sprintf("begin%d", attempt))
				if attempt > tc.failures {
					expected = append(expected, fmt.Sprintf("commit%d", attempt))
				}
				expected = append(expected, fmt.Sprintf("rollback%d", attempt))
			}
			assert.Equal(t, expected, events)
			if tc.attempts > tc.failures {
				require.NoError(t, err)
				require.NotNil(t, got)
			} else {
				require.ErrorIs(t, err, tc.cause)
				assert.Nil(t, got)
				if _, ok := tc.cause.(*pgconn.PgError); ok {
					var cause *pgconn.PgError
					require.ErrorAs(t, err, &cause)
					assert.Same(t, tc.cause, cause)
				}
			}
		})
	}
}
