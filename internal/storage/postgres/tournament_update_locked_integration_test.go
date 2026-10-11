package postgres

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	transport "holdem-tournament-builder/internal/transport/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentRepository_UpdateLockedIntegration(t *testing.T) {
	t.Run("concurrent HTTP patches preserve both player payments", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		tr.Players[0].BuyInPaid, tr.Players[1].BuyInPaid = false, false
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, err)
		t.Cleanup(func() { rollbackTournamentTestTx(tx) })
		var lockedID uuid.UUID
		require.NoError(t, tx.QueryRow(ctx, "SELECT id FROM tournaments WHERE id = $1 FOR UPDATE", id).Scan(&lockedID))
		router := transport.NewRouter(transport.NewTournamentHandler(service.NewTournamentService(repo)))
		workerCtx, cancel := context.WithCancel(ctx)
		var workers sync.WaitGroup
		t.Cleanup(func() {
			cancel()
			rollbackTournamentTestTx(tx)
			workers.Wait()
		})
		responses := make(chan *httptest.ResponseRecorder, 2)
		for _, player := range tr.Players {
			workers.Add(1)
			go func() {
				defer workers.Done()
				path := "/tournaments/" + id.String() + "/players/" + player.ID.String()
				req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"buy_in_paid":true}`)).WithContext(workerCtx)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				responses <- rec
			}()
		}
		// Both requests must reach the database before either can save a card.
		require.Eventually(t, func() bool {
			var waiting int
			err := pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM pg_stat_activity
				WHERE datname = current_database()
				  AND pid <> pg_backend_pid()
				  AND wait_event_type = 'Lock'
				  AND query LIKE '%tournaments%'
			`).Scan(&waiting)
			return err == nil && waiting == 2
		}, 2*time.Second, 10*time.Millisecond)
		require.NoError(t, tx.Rollback(ctx))
		for range 2 {
			rec := <-responses
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}
		got, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		tr.Players[0].BuyInPaid, tr.Players[1].BuyInPaid = true, true
		assert.Equal(t, tr, got)
	})
	t.Run("serializes changes to one card without blocking another card", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		tr.Players[0].BuyInPaid, tr.Players[1].BuyInPaid = false, false
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		other := validTournament(t)
		otherID, err := repo.Create(ctx, other)
		require.NoError(t, err)

		locked := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		releaseFirst := func() { releaseOnce.Do(func() { close(release) }) }
		var workers sync.WaitGroup
		workerCtx, cancel := context.WithCancel(ctx)
		t.Cleanup(func() {
			cancel()
			releaseFirst()
			workers.Wait()
		})
		firstDone, secondDone := make(chan error, 1), make(chan error, 1)
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := repo.UpdateLocked(workerCtx, id, func(current *domain.Tournament) (*domain.Tournament, error) {
				close(locked)
				select {
				case <-release:
					current.Players[0].BuyInPaid = true
					return current, nil
				case <-workerCtx.Done():
					return nil, workerCtx.Err()
				}
			})
			firstDone <- err
		}()
		select {
		case <-locked:
		case err := <-firstDone:
			t.Fatalf("first update did not acquire the row: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := repo.UpdateLocked(workerCtx, id, func(current *domain.Tournament) (*domain.Tournament, error) {
				if !current.Players[0].BuyInPaid {
					return nil, errors.New("second update read an outdated card")
				}
				current.Players[1].BuyInPaid = true
				return current, nil
			})
			secondDone <- err
		}()

		// Observe a real database lock wait rather than depending on a sleep.
		require.Eventually(t, func() bool {
			var waiting bool
			err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM pg_stat_activity
					WHERE datname = current_database()
					  AND pid <> pg_backend_pid()
					  AND wait_event_type = 'Lock'
					  AND query LIKE '%FOR UPDATE%'
				)
			`).Scan(&waiting)
			return err == nil && waiting
		}, 2*time.Second, 10*time.Millisecond)
		select {
		case err := <-secondDone:
			t.Fatalf("second update must wait for the first: %v", err)
		default:
		}
		updatedOther, err := repo.UpdateLocked(ctx, otherID, func(current *domain.Tournament) (*domain.Tournament, error) {
			current.Name = "Independent Game"
			return current, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "Independent Game", updatedOther.Name)
		releaseFirst()
		require.NoError(t, <-firstDone)
		require.NoError(t, <-secondDone)

		got, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		tr.Players[0].BuyInPaid, tr.Players[1].BuyInPaid = true, true
		assert.Equal(t, tr, got)
		storedOther, err := repo.GetByID(ctx, otherID)
		require.NoError(t, err)
		other.Name = "Independent Game"
		assert.Equal(t, other, storedOther)
	})
	t.Run("rolls back rejected change and permits the next update", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		cause := errors.New("change rejected")
		got, err := repo.UpdateLocked(ctx, id, func(current *domain.Tournament) (*domain.Tournament, error) {
			current.Name = "Must Not Be Saved"
			current.Players[0].BuyInPaid = true
			return current, cause
		})
		require.ErrorIs(t, err, cause)
		assert.Nil(t, got)
		stored, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, tr, stored)
		got, err = repo.UpdateLocked(ctx, id, func(current *domain.Tournament) (*domain.Tournament, error) {
			current.Name = "Next Game"
			return current, nil
		})
		require.NoError(t, err)
		tr.Name = "Next Game"
		assert.Equal(t, tr, got)
		stored, err = repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, tr, stored)
	})
	t.Run("cancels lock wait without changing the card", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)
		id, err := repo.Create(ctx, tr)
		require.NoError(t, err)
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, err)
		t.Cleanup(func() { rollbackTournamentTestTx(tx) })
		var lockedID uuid.UUID
		require.NoError(t, tx.QueryRow(ctx, "SELECT id FROM tournaments WHERE id = $1 FOR UPDATE", id).Scan(&lockedID))
		waitCtx, cancel := context.WithCancel(ctx)
		var workers sync.WaitGroup
		t.Cleanup(func() {
			cancel()
			rollbackTournamentTestTx(tx)
			workers.Wait()
		})
		called := false
		type result struct {
			card *domain.Tournament
			err  error
		}
		done := make(chan result, 1)
		workers.Add(1)
		go func() {
			defer workers.Done()
			got, err := repo.UpdateLocked(waitCtx, id, func(current *domain.Tournament) (*domain.Tournament, error) {
				called = true
				current.Name = "Must Not Be Saved"
				return current, nil
			})
			done <- result{got, err}
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM pg_stat_activity
					WHERE datname = current_database()
					  AND pid <> pg_backend_pid()
					  AND wait_event_type = 'Lock'
					  AND query LIKE '%FOR UPDATE%'
				)
			`).Scan(&waiting)
			return err == nil && waiting
		}, 2*time.Second, 10*time.Millisecond)
		cancel()
		cancelled := <-done
		require.ErrorIs(t, cancelled.err, context.Canceled)
		assert.Nil(t, cancelled.card)
		assert.False(t, called)
		require.NoError(t, tx.Rollback(ctx))
		stored, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, tr, stored)
		got, err := repo.UpdateLocked(ctx, id, func(current *domain.Tournament) (*domain.Tournament, error) {
			current.Players[0].BuyInPaid = true
			return current, nil
		})
		require.NoError(t, err)
		assert.True(t, got.Players[0].BuyInPaid)
	})
	t.Run("missing tournament never invokes change", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		called := false
		got, err := repo.UpdateLocked(ctx, uuid.New(), func(current *domain.Tournament) (*domain.Tournament, error) {
			called = true
			return current, nil
		})
		require.ErrorIs(t, err, app.ErrTournamentNotFound)
		assert.Nil(t, got)
		assert.False(t, called)
	})
}

func rollbackTournamentTestTx(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
