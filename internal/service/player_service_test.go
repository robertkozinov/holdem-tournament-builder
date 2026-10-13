package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type playerRepoStub struct {
	owner         uuid.UUID
	id            uuid.UUID
	player        domain.Player
	query         string
	limit, offset int
	called        bool
	err           error
}

func (r *playerRepoStub) Create(_ context.Context, owner uuid.UUID, p domain.Player) (domain.Player, error) {
	r.called, r.owner, r.player = true, owner, p
	return p, r.err
}
func (r *playerRepoStub) List(_ context.Context, owner uuid.UUID, query string, limit, offset int) ([]domain.Player, error) {
	r.called, r.owner, r.query, r.limit, r.offset = true, owner, query, limit, offset
	return []domain.Player{r.player}, r.err
}
func (r *playerRepoStub) Rename(_ context.Context, owner, id uuid.UUID, name, normalized string, now time.Time) (domain.Player, error) {
	r.called, r.owner, r.id = true, owner, id
	r.player = domain.Player{ID: id, Name: name, NormalizedName: normalized, UpdatedAt: now}
	return r.player, r.err
}
func (r *playerRepoStub) Delete(_ context.Context, owner, id uuid.UUID, now time.Time) error {
	r.called, r.owner, r.id = true, owner, id
	r.player.DeletedAt = &now
	return r.err
}

func TestPlayerService_Create(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	owner := uuid.New()
	repo := &playerRepoStub{}
	player, err := NewPlayerService(repo).CreatePlayer(app.WithUserID(context.Background(), owner), "  Влад  Фед ", now)
	require.NoError(t, err)
	assert.Equal(t, owner, repo.owner)
	assert.NotEqual(t, uuid.Nil, player.ID)
	assert.Equal(t, "Влад Фед", player.Name)
	assert.Equal(t, "влад фед", player.NormalizedName)
	assert.Equal(t, now, player.CreatedAt)
	assert.Equal(t, now, player.UpdatedAt)
	assert.Nil(t, player.DeletedAt)
}

func TestPlayerService_List(t *testing.T) {
	repo := &playerRepoStub{}
	owner := uuid.New()
	_, err := NewPlayerService(repo).ListPlayers(app.WithUserID(context.Background(), owner), " ФЕД ", 10, 20)
	require.NoError(t, err)
	assert.Equal(t, owner, repo.owner)
	assert.Equal(t, "фед", repo.query)
	assert.Equal(t, 10, repo.limit)
	assert.Equal(t, 20, repo.offset)
}

func TestPlayerService_RenameAndDelete(t *testing.T) {
	now := time.Now().UTC()
	owner, id := uuid.New(), uuid.New()
	ctx := app.WithUserID(context.Background(), owner)
	repo := &playerRepoStub{}
	s := NewPlayerService(repo)
	p, err := s.RenamePlayer(ctx, id, " ВЛАД Поп ", now)
	require.NoError(t, err)
	assert.Equal(t, id, p.ID)
	assert.Equal(t, "ВЛАД Поп", p.Name)
	assert.Equal(t, "влад поп", p.NormalizedName)
	assert.Equal(t, owner, repo.owner)
	require.NoError(t, s.DeletePlayer(ctx, id, now))
	assert.Equal(t, id, repo.id)
	assert.Equal(t, now, *repo.player.DeletedAt)
}

func TestPlayerService_RejectsBeforeStorage(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*PlayerService) error
		want error
	}{
		{"create unauthenticated", func(s *PlayerService) error {
			_, e := s.CreatePlayer(context.Background(), "Влад", time.Now())
			return e
		}, app.ErrUnauthenticated},
		{"list unauthenticated", func(s *PlayerService) error { _, e := s.ListPlayers(context.Background(), "", 10, 0); return e }, app.ErrUnauthenticated},
		{"rename unauthenticated", func(s *PlayerService) error {
			_, e := s.RenamePlayer(context.Background(), uuid.New(), "Влад", time.Now())
			return e
		}, app.ErrUnauthenticated},
		{"delete unauthenticated", func(s *PlayerService) error { return s.DeletePlayer(context.Background(), uuid.New(), time.Now()) }, app.ErrUnauthenticated},
		{"blank name", func(s *PlayerService) error {
			_, e := s.CreatePlayer(app.WithUserID(context.Background(), uuid.New()), " ", time.Now())
			return e
		}, app.ErrValidation},
		{"nil rename id", func(s *PlayerService) error {
			_, e := s.RenamePlayer(app.WithUserID(context.Background(), uuid.New()), uuid.Nil, "Влад", time.Now())
			return e
		}, app.ErrInvalidPlayerID},
		{"nil delete id", func(s *PlayerService) error {
			return s.DeletePlayer(app.WithUserID(context.Background(), uuid.New()), uuid.Nil, time.Now())
		}, app.ErrInvalidPlayerID},
		{"invalid page", func(s *PlayerService) error {
			_, e := s.ListPlayers(app.WithUserID(context.Background(), uuid.New()), "", 101, 0)
			return e
		}, app.ErrValidation},
		{"invalid offset", func(s *PlayerService) error {
			_, e := s.ListPlayers(app.WithUserID(context.Background(), uuid.New()), "", 1, -1)
			return e
		}, app.ErrValidation},
		{"long query", func(s *PlayerService) error {
			_, e := s.ListPlayers(app.WithUserID(context.Background(), uuid.New()), strings.Repeat("a", 101), 1, 0)
			return e
		}, app.ErrValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &playerRepoStub{}
			require.ErrorIs(t, tc.call(NewPlayerService(r)), tc.want)
			assert.False(t, r.called)
		})
	}
}

func TestPlayerService_PreservesErrors(t *testing.T) {
	ctx := app.WithUserID(context.Background(), uuid.New())
	for _, cause := range []error{app.ErrPlayerNameTaken, app.ErrPlayerNotFound, errors.New("storage failed")} {
		r := &playerRepoStub{err: cause}
		s := NewPlayerService(r)
		_, err := s.CreatePlayer(ctx, "Влад", time.Now())
		require.ErrorIs(t, err, cause)
		_, err = s.ListPlayers(ctx, "", 10, 0)
		require.ErrorIs(t, err, cause)
		_, err = s.RenamePlayer(ctx, uuid.New(), "Фед", time.Now())
		require.ErrorIs(t, err, cause)
		require.ErrorIs(t, s.DeletePlayer(ctx, uuid.New(), time.Now()), cause)
	}
}
