package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"holdem-tournament-builder/internal/app"
	"strings"
	"testing"

	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tournamentPlayerSaver interface {
	SaveWithPlayers(context.Context, uuid.UUID, uuid.UUID, []domain.TournamentPlayer, func([]domain.TournamentPlayer, *domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error)
}

type registryTestPlayer struct {
	id               uuid.UUID
	name, normalized string
	deleted          bool
}
type registryTestRows struct {
	pgx.Rows
	players []registryTestPlayer
	index   int
}

func (r *registryTestRows) Close()     {}
func (r *registryTestRows) Err() error { return nil }
func (r *registryTestRows) Next() bool { r.index++; return r.index < len(r.players) }
func (r *registryTestRows) Scan(dest ...any) error {
	p := r.players[r.index]
	*(dest[0].(*uuid.UUID)) = p.id
	if len(dest) == 2 {
		*(dest[1].(*string)) = p.normalized
		return nil
	}
	*(dest[1].(*string)) = p.name
	*(dest[2].(*string)) = p.normalized
	*(dest[3].(*bool)) = p.deleted
	return nil
}
func registryRows(players ...registryTestPlayer) pgx.Rows {
	return &registryTestRows{players: players, index: -1}
}

func TestTournamentRepositoryHydratesRegistryAndRejectsMissingIDs(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "current profile", true: "missing profile"}[missing], func(t *testing.T) {
			tr := validTournament(t)
			tr.ID = uuid.New()
			data, err := json.Marshal(tr)
			require.NoError(t, err)
			queries := 0
			db := &stubTournamentDB{
				queryRow: func(context.Context, string, ...any) pgx.Row {
					return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*[]byte)) = data; return nil })
				},
				query: func(_ context.Context, query string, args ...any) (pgx.Rows, error) {
					queries++
					assert.Contains(t, query, "user_id = $1")
					assert.Equal(t, testOwnerID, args[0])
					if missing {
						return registryRows(), nil
					}
					return registryRows(registryTestPlayer{tr.Players[0].ID, "Current", "current", true}, registryTestPlayer{tr.Players[1].ID, "B", "b", false}), nil
				},
			}
			got, err := (&TournamentRepository{pool: db}).GetByID(context.Background(), testOwnerID, tr.ID)
			if missing {
				require.Error(t, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "Current", got.Players[0].Name)
				assert.True(t, got.Players[0].IsDeleted)
			}
			assert.Equal(t, 1, queries)
		})
	}
}

func TestTournamentRepositoryAtomicRegistryRollback(t *testing.T) {
	for _, stage := range []string{"build", "write", "links", "commit", "success"} {
		t.Run(stage, func(t *testing.T) {
			cause := errors.New("rejected")
			id := uuid.New()
			var events []string
			tx := &stubTournamentTx{
				query: func(context.Context, string, ...any) (pgx.Rows, error) { return registryRows(), nil },
				queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
					assert.Contains(t, query, "INSERT INTO players")
					assert.Equal(t, testOwnerID, args[1])
					assert.Equal(t, "Влад Фед", args[2])
					assert.Equal(t, "влад фед", args[3])
					events = append(events, "player")
					return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*uuid.UUID)) = id; return nil })
				},
				exec: func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
					if strings.Contains(query, "INSERT INTO tournaments") {
						events = append(events, "card")
						if stage == "write" {
							return pgconn.CommandTag{}, cause
						}
					}
					if strings.Contains(query, "INSERT INTO tournament_players") {
						events = append(events, "links")
						if stage == "links" {
							return pgconn.CommandTag{}, cause
						}
					}
					return pgconn.NewCommandTag("INSERT 0 1"), nil
				},
				commit: func(context.Context) error {
					events = append(events, "commit")
					if stage == "commit" {
						return cause
					}
					return nil
				},
				rollback: func(context.Context) error { events = append(events, "rollback"); return nil },
			}
			db := &stubTournamentDB{beginTx: func(context.Context, pgx.TxOptions) (pgx.Tx, error) { return tx, nil }}
			saver, ok := any(&TournamentRepository{pool: db}).(tournamentPlayerSaver)
			require.True(t, ok, "missing atomic boundary")
			got, err := saver.SaveWithPlayers(context.Background(), testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "  Влад\tФед "}}, func(resolved []domain.TournamentPlayer, original *domain.Tournament) (*domain.Tournament, error) {
				events = append(events, "build")
				assert.Nil(t, original)
				require.Len(t, resolved, 1)
				assert.Equal(t, id, resolved[0].ID)
				assert.Equal(t, "Влад Фед", resolved[0].Name)
				if stage == "build" {
					return nil, cause
				}
				tr := validTournament(t)
				tr.Players = resolved
				return tr, nil
			})
			if stage == "success" {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Contains(t, events, "commit")
			} else {
				require.ErrorIs(t, err, cause)
				assert.Nil(t, got)
				if stage != "commit" {
					assert.NotContains(t, events, "commit")
				}
			}
			assert.Equal(t, "rollback", events[len(events)-1])
		})
	}
}

func TestTournamentConcreteWritesCannotBypassRegistry(t *testing.T) {
	for _, operation := range []string{"create", "update", "patch"} {
		t.Run(operation, func(t *testing.T) {
			tr := validTournament(t)
			tr.ID = uuid.New()
			data, err := json.Marshal(tr)
			require.NoError(t, err)
			writes := 0
			exec := func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				writes++
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			tx := &stubTournamentTx{
				exec:  exec,
				query: func(context.Context, string, ...any) (pgx.Rows, error) { return registryRows(), nil },
				queryRow: func(context.Context, string, ...any) pgx.Row {
					return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*[]byte)) = data; return nil })
				},
				commit: func(context.Context) error { return nil }, rollback: func(context.Context) error { return nil },
			}
			db := &stubTournamentDB{exec: exec, beginTx: func(context.Context, pgx.TxOptions) (pgx.Tx, error) { return tx, nil }}
			repo := &TournamentRepository{pool: db}
			switch operation {
			case "create":
				_, err = repo.Create(context.Background(), testOwnerID, tr)
			case "update":
				err = repo.Update(context.Background(), testOwnerID, tr)
			case "patch":
				_, err = repo.UpdateLocked(context.Background(), testOwnerID, tr.ID, func(card *domain.Tournament) (*domain.Tournament, error) { return card, nil })
			}
			require.ErrorIs(t, err, app.ErrPlayerNotFound)
			assert.Zero(t, writes)
		})
	}
}

func TestTournamentRepositoryHasAtomicPlayerBoundary(t *testing.T) {
	_, ok := any(NewTournamentRepository(nil)).(tournamentPlayerSaver)
	require.True(t, ok, "tournament repository must resolve registry players and build the card in one transaction")
}
