package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registryTournamentBuilder(template *domain.Tournament) func([]domain.TournamentPlayer, *domain.Tournament) (*domain.Tournament, error) {
	return func(players []domain.TournamentPlayer, original *domain.Tournament) (*domain.Tournament, error) {
		candidate := *template
		candidate.ID, candidate.Players, candidate.Results = uuid.Nil, players, nil
		if original != nil {
			candidate.ID, candidate.BuyInAmount = original.ID, original.BuyInAmount
		}
		card, err := domain.NewTournament(candidate)
		if err != nil {
			return nil, errors.Join(app.ErrValidation, err)
		}
		return card, nil
	}
}

func TestTournamentRegistryIntegration(t *testing.T) {
	t.Run("exact normalized reuse and transactional rollback", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		build := registryTournamentBuilder(validTournament(t))
		first, err := repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "  Влад\tФед "}, {Name: "B"}}, build)
		require.NoError(t, err)
		assert.Equal(t, "Влад Фед", first.Players[0].Name)
		second, err := repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "ВЛАД ФЕД"}, {Name: "Влад"}}, build)
		require.NoError(t, err)
		assert.Equal(t, first.Players[0].ID, second.Players[0].ID)
		assert.NotEqual(t, first.Players[0].ID, second.Players[1].ID)
		var profiles, links int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM players WHERE user_id=$1", testOwnerID).Scan(&profiles))
		assert.Equal(t, 3, profiles)
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM tournament_players WHERE user_id=$1", testOwnerID).Scan(&links))
		assert.Equal(t, 4, links)
		_, err = repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "Fresh"}, {Name: "  FRESH "}}, build)
		require.ErrorIs(t, err, domain.ErrDuplicatePlayer)
		invalid := validTournament(t)
		invalid.Name = ""
		_, err = repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "New A"}, {Name: "New B"}}, registryTournamentBuilder(invalid))
		require.ErrorIs(t, err, domain.ErrEmptyName)
		duplicate := validTournament(t)
		duplicate.ID = first.ID
		duplicate.Results = nil
		duplicate.Players = []domain.TournamentPlayer{{Name: "Rollback A"}, {Name: "Rollback B"}}
		_, err = repo.Create(ctx, testOwnerID, duplicate)
		var pgErr interface{ SQLState() string }
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, "23505", pgErr.SQLState())
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM players WHERE user_id=$1", testOwnerID).Scan(&profiles))
		assert.Equal(t, 3, profiles)
		require.NoError(t, repo.Delete(ctx, testOwnerID, first.ID))
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM tournament_players WHERE tournament_id=$1", first.ID).Scan(&links))
		assert.Zero(t, links)
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM players WHERE user_id=$1", testOwnerID).Scan(&profiles))
		assert.Equal(t, 3, profiles)
	})
	t.Run("selected identity ownership rename delete and recreate", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		build := registryTournamentBuilder(validTournament(t))
		card, err := repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "A"}, {Name: "B"}}, build)
		require.NoError(t, err)
		profiles := NewPlayerRepository(pool)
		id := card.Players[0].ID
		_, err = profiles.Rename(ctx, testOwnerID, id, "Renamed", "renamed", time.Now().UTC())
		require.NoError(t, err)
		got, err := repo.GetByID(ctx, testOwnerID, card.ID)
		require.NoError(t, err)
		assert.Equal(t, "Renamed", got.Players[0].Name)
		page, err := repo.List(ctx, testOwnerID, 50, 0)
		require.NoError(t, err)
		assert.Equal(t, "Renamed", page[0].Players[0].Name)
		card.Players[0].Name = "spoof"
		card.Players[0].IsDeleted = true
		saved, err := repo.SaveWithPlayers(ctx, testOwnerID, card.ID, card.Players, build)
		require.NoError(t, err)
		assert.Equal(t, "Renamed", saved.Players[0].Name)
		assert.False(t, saved.Players[0].IsDeleted)
		require.NoError(t, profiles.Delete(ctx, testOwnerID, id, time.Now().UTC()))
		paid := true
		patched, err := service.NewTournamentService(repo).PatchTournamentPlayer(app.WithUserID(ctx, testOwnerID), card.ID, id, service.PatchTournamentPlayerInput{BuyInPaid: &paid})
		require.NoError(t, err)
		assert.True(t, patched.Players[0].IsDeleted)
		assert.True(t, patched.Players[0].BuyInPaid)
		saved, err = repo.SaveWithPlayers(ctx, testOwnerID, card.ID, patched.Players, build)
		require.NoError(t, err)
		assert.True(t, saved.Players[0].IsDeleted)
		_, err = repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, patched.Players, build)
		require.ErrorIs(t, err, app.ErrPlayerNotFound)
		_, err = repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{ID: uuid.New(), Name: "No auto create"}, {Name: "Other"}}, build)
		require.ErrorIs(t, err, app.ErrPlayerNotFound)
		other := domain.User{ID: uuid.New(), Login: "foreign_" + uuid.NewString()[:8], PasswordHash: "test-hash", CreatedAt: time.Now().UTC()}
		require.NoError(t, NewUserRepository(pool).Create(ctx, other))
		_, err = repo.SaveWithPlayers(ctx, other.ID, uuid.Nil, []domain.TournamentPlayer{{ID: card.Players[1].ID}, {Name: "Foreign"}}, build)
		require.ErrorIs(t, err, app.ErrPlayerNotFound)
		_, err = repo.SaveWithPlayers(ctx, other.ID, card.ID, card.Players, build)
		require.ErrorIs(t, err, app.ErrTournamentNotFound)
		recreated, err := repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "Renamed"}, {Name: "B"}}, build)
		require.NoError(t, err)
		assert.NotEqual(t, id, recreated.Players[0].ID)
		selections := []domain.TournamentPlayer{{ID: id}, {ID: recreated.Players[0].ID}}
		withNames, err := repo.SaveWithPlayers(ctx, testOwnerID, card.ID, selections, build)
		require.NoError(t, err)
		assert.Equal(t, withNames.Players[0].Name, withNames.Players[1].Name)
		withNames.Results = []domain.Result{{PlayerID: id, Prize: withNames.Pot()}}
		require.Equal(t, []domain.Transfer{{From: recreated.Players[0].ID, To: id, Amount: 1000}}, withNames.CalculateTransfers())
		_, err = repo.SaveWithPlayers(ctx, testOwnerID, card.ID, []domain.TournamentPlayer{{ID: recreated.Players[0].ID}, {Name: "B"}}, build)
		require.NoError(t, err)
		_, err = repo.SaveWithPlayers(ctx, testOwnerID, card.ID, selections, build)
		require.ErrorIs(t, err, app.ErrPlayerNotFound)
	})
	t.Run("concurrent exact names create one profile per key", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		build := registryTournamentBuilder(validTournament(t))
		var workers sync.WaitGroup
		errors := make(chan error, 8)
		ids := make(chan uuid.UUID, 8)
		for range 8 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				card, err := repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "Concurrent A"}, {Name: "Concurrent B"}}, build)
				errors <- err
				if err == nil {
					ids <- card.Players[0].ID
				}
			}()
		}
		workers.Wait()
		close(errors)
		close(ids)
		for err := range errors {
			require.NoError(t, err)
		}
		var common uuid.UUID
		for id := range ids {
			if common == uuid.Nil {
				common = id
			}
			assert.Equal(t, common, id)
		}
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM players WHERE user_id=$1", testOwnerID).Scan(&count))
		assert.Equal(t, 2, count)
	})
}

func TestTournamentRegistryProfileRaces(t *testing.T) {
	for _, operation := range []string{"rename", "delete"} {
		t.Run(operation, func(t *testing.T) {
			ctx, pool, repo := setupTournamentRepositoryTest(t)
			build := registryTournamentBuilder(validTournament(t))
			card, err := repo.SaveWithPlayers(ctx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "Race A"}, {Name: "Race B"}}, build)
			require.NoError(t, err)
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { rollbackTournamentTestTx(tx) })
			if operation == "rename" {
				_, err = tx.Exec(ctx, "UPDATE players SET name='Race Renamed',normalized_name='race renamed' WHERE id=$1", card.Players[0].ID)
			} else {
				_, err = tx.Exec(ctx, "UPDATE players SET deleted_at=NOW() WHERE id=$1", card.Players[0].ID)
			}
			require.NoError(t, err)
			type result struct {
				card *domain.Tournament
				err  error
			}
			done := make(chan result, 1)
			workerCtx, cancel := context.WithCancel(ctx)
			var workers sync.WaitGroup
			t.Cleanup(func() { cancel(); rollbackTournamentTestTx(tx); workers.Wait() })
			workers.Add(1)
			go func() {
				defer workers.Done()
				saved, err := repo.SaveWithPlayers(workerCtx, testOwnerID, uuid.Nil, []domain.TournamentPlayer{{Name: "Race A"}, {Name: "Race B"}}, build)
				done <- result{saved, err}
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FOR SHARE%')`).Scan(&waiting)
				return err == nil && waiting
			}, 2*time.Second, 10*time.Millisecond)
			require.NoError(t, tx.Commit(ctx))
			out := <-done
			require.NoError(t, out.err)
			assert.NotEqual(t, card.Players[0].ID, out.card.Players[0].ID)
			assert.Equal(t, "Race A", out.card.Players[0].Name)
			assert.False(t, out.card.Players[0].IsDeleted)
			var count int
			require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM players WHERE user_id=$1", testOwnerID).Scan(&count))
			assert.Equal(t, 3, count)
		})
	}
}
