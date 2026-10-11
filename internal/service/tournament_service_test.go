package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"testing"
	"time"
)

var testTournamentID = uuid.MustParse("00000000-0000-0000-0000-000000000010")

type mockTournamentRepository struct {
	createCalled        bool
	createdTournament   *domain.Tournament
	createID            uuid.UUID
	createErr           error
	getCalled           bool
	getCount            int
	getID               uuid.UUID
	getTournament       *domain.Tournament
	getErr              error
	updateCalled        bool
	updateCount         int
	ordinaryUpdateCount int
	updatedTournament   *domain.Tournament
	updateErr           error
	lockedCalled        bool
	lockedCount         int
	lockedID            uuid.UUID
	lockedErr           error
	deleteCalled        bool
	deleteID            uuid.UUID
	deleteErr           error
	listCalled          bool
	limit, offset       int
	tournaments         []*domain.Tournament
	listErr             error
}

func (r *mockTournamentRepository) Create(ctx context.Context, tr *domain.Tournament) (uuid.UUID, error) {
	r.createCalled = true
	r.createdTournament = tr
	if r.createErr != nil {
		return uuid.Nil, r.createErr
	}
	tr.ID = r.createID
	return r.createID, nil
}
func (r *mockTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	r.getCalled = true
	r.getCount++
	r.getID = id
	return r.getTournament, r.getErr
}
func (r *mockTournamentRepository) Update(ctx context.Context, tr *domain.Tournament) error {
	r.updateCalled = true
	r.updateCount++
	r.ordinaryUpdateCount++
	r.updatedTournament = tr
	if r.updateErr != nil {
		return r.updateErr
	}
	r.getTournament = tr
	return nil
}
func (r *mockTournamentRepository) UpdateLocked(ctx context.Context, id uuid.UUID, change func(*domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error) {
	r.lockedCalled = true
	r.lockedCount++
	r.lockedID = id
	r.getID = id
	if r.getErr != nil {
		return nil, r.getErr
	}
	updated, err := change(r.getTournament)
	if err != nil {
		return nil, err
	}
	r.updateCount++
	r.updatedTournament = updated
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	if r.lockedErr != nil {
		return nil, r.lockedErr
	}
	r.getTournament = updated
	return updated, nil
}
func (r *mockTournamentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.deleteCalled = true
	r.deleteID = id
	return r.deleteErr
}
func (r *mockTournamentRepository) List(ctx context.Context, limit, offset int) ([]*domain.Tournament, error) {
	r.listCalled = true
	r.limit = limit
	r.offset = offset
	return r.tournaments, r.listErr
}

func validSaveTournamentInput() SaveTournamentInput {
	return SaveTournamentInput{
		Name: "Friday Game", Date: time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC),
		Players:        []domain.TournamentPlayer{{ID: uuid.New(), Name: "A"}, {ID: uuid.New(), Name: "B"}, {ID: uuid.New(), Name: "C"}, {ID: uuid.New(), Name: "D"}},
		ChipSet:        []domain.ChipDenomination{{Value: 25, Count: 80}, {Value: 100, Count: 80}, {Value: 500, Count: 40}},
		SettlementMode: domain.SettlementP2P, Style: domain.StyleStandard, Duration: 2 * time.Hour, LevelDuration: 20 * time.Minute,
	}
}
func validCreateTournamentInput() CreateTournamentInput {
	in := validSaveTournamentInput()
	return CreateTournamentInput{Name: in.Name, Date: in.Date, Players: in.Players, BuyInAmount: 1000, SettlementMode: in.SettlementMode, ChipSet: in.ChipSet, Style: in.Style, Duration: in.Duration, LevelDuration: in.LevelDuration}
}
func serviceTournamentFixture(t *testing.T, in SaveTournamentInput) *domain.Tournament {
	t.Helper()
	tr, err := domain.NewTournament(domain.Tournament{ID: testTournamentID, Name: in.Name, Date: in.Date, Players: in.Players, BuyInAmount: 1000, SettlementMode: in.SettlementMode, ChipSet: in.ChipSet, Style: in.Style, Duration: in.Duration, LevelDuration: in.LevelDuration, Results: in.Results})
	require.NoError(t, err)
	return tr
}

func TestTournamentService_CreateTournament(t *testing.T) {
	t.Run("creates draft and generates missing ids without changing input", func(t *testing.T) {
		in := validCreateTournamentInput()
		in.Name = " Friday Game "
		in.Players[0].Name = " A "
		in.Players[0].ID = uuid.Nil
		in.Players[0].Rebuys = []domain.Rebuy{{Paid: true}}
		repo := &mockTournamentRepository{createID: testTournamentID}
		id, err := NewTournamentService(repo).CreateTournament(context.Background(), in)
		require.NoError(t, err)
		assert.Equal(t, testTournamentID, id)
		tr := repo.createdTournament
		require.NotNil(t, tr)
		assert.Equal(t, "Friday Game", tr.Name)
		assert.Equal(t, "A", tr.Players[0].Name)
		assert.NotEqual(t, uuid.Nil, tr.Players[0].ID)
		assert.NotEqual(t, uuid.Nil, tr.Players[0].Rebuys[0].ID)
		assert.True(t, tr.Players[0].Rebuys[0].Paid)
		assert.Equal(t, int64(5000), tr.Pot())
		assert.NotEmpty(t, tr.StartingStack.Distribution)
		assert.Len(t, tr.BlindStructure, 6)
		assert.Empty(t, tr.Results)
		assert.Equal(t, uuid.Nil, in.Players[0].ID)
		assert.Equal(t, " A ", in.Players[0].Name)
		assert.Equal(t, uuid.Nil, in.Players[0].Rebuys[0].ID)
	})
	tests := []struct {
		name    string
		change  func(*CreateTournamentInput)
		wantErr error
	}{
		{"empty name", func(in *CreateTournamentInput) { in.Name = "" }, domain.ErrEmptyName},
		{"empty player name", func(in *CreateTournamentInput) { in.Players[0].Name = " " }, domain.ErrInvalidPlayerName},
		{"duplicate normalized names", func(in *CreateTournamentInput) { in.Players[1].Name = " A " }, domain.ErrDuplicatePlayer},
		{"empty chip set", func(in *CreateTournamentInput) { in.ChipSet = nil }, domain.ErrEmptyChipSet},
		{"small stack", func(in *CreateTournamentInput) { in.ChipSet = []domain.ChipDenomination{{Value: 1, Count: 1}} }, domain.ErrSmallStack},
		{"too few blind levels", func(in *CreateTournamentInput) { in.Duration = 20 * time.Minute }, domain.ErrNotEnoughLevels},
		{"invalid mode", func(in *CreateTournamentInput) { in.SettlementMode = "unknown" }, domain.ErrInvalidSettlementMode},
		{"invalid style", func(in *CreateTournamentInput) { in.Style = 99 }, domain.ErrInvalidStyle},
		{"too many blind levels", func(in *CreateTournamentInput) { in.Duration = 1001 * time.Minute; in.LevelDuration = time.Minute }, domain.ErrTooManyBlindLevels},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validCreateTournamentInput()
			tt.change(&in)
			repo := &mockTournamentRepository{}
			id, err := NewTournamentService(repo).CreateTournament(context.Background(), in)
			require.ErrorIs(t, err, app.ErrValidation)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, uuid.Nil, id)
			assert.False(t, repo.createCalled)
		})
	}
	t.Run("repository error stays identifiable", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{createErr: repoErr}
		id, err := NewTournamentService(repo).CreateTournament(context.Background(), validCreateTournamentInput())
		require.ErrorIs(t, err, repoErr)
		assert.NotErrorIs(t, err, app.ErrValidation)
		assert.Equal(t, uuid.Nil, id)
	})
}

func TestTournamentService_SaveTournament(t *testing.T) {
	t.Run("repeated saves replace results and preserve buy in", func(t *testing.T) {
		in := validSaveTournamentInput()
		original := serviceTournamentFixture(t, in)
		repo := &mockTournamentRepository{getTournament: original}
		srv := NewTournamentService(repo)
		in.Name = "Changed"
		in.SettlementMode = domain.SettlementBank
		in.Players[0].BuyInPaid = true
		in.Players[0].Rebuys = []domain.Rebuy{{ID: uuid.New(), Paid: false}, {ID: uuid.New(), Paid: true}}
		in.Results = []domain.Result{{PlayerID: in.Players[0].ID, Prize: 3700}, {PlayerID: in.Players[1].ID, Prize: 2300}}
		first, err := srv.SaveTournament(context.Background(), testTournamentID, in)
		require.NoError(t, err)
		assert.Equal(t, testTournamentID, first.ID)
		assert.Equal(t, int64(1000), first.BuyInAmount)
		assert.Equal(t, int64(6000), first.Pot())
		assert.True(t, first.HasResults())
		assert.Nil(t, first.Results[0].Place)
		assert.Len(t, first.Results, 2)
		assert.Empty(t, first.CalculateTransfers())
		assert.NotEmpty(t, first.StartingStack.Distribution)
		second, err := srv.SaveTournament(context.Background(), testTournamentID, in)
		require.NoError(t, err)
		assert.Equal(t, first, second)
		assert.Equal(t, 2, repo.updateCount)
		assert.Equal(t, 2, repo.getCount)
		assert.Equal(t, 2, repo.ordinaryUpdateCount)
		assert.False(t, repo.lockedCalled)
		assert.Zero(t, repo.lockedCount)
		assert.Len(t, second.Players[0].Rebuys, 2)
		assert.Equal(t, "Friday Game", original.Name)
		assert.Empty(t, original.Results)
		assert.Empty(t, original.Players[0].Rebuys)
		place := 2
		in.Results = []domain.Result{{PlayerID: in.Players[2].ID, Place: &place, Prize: 6000}}
		corrected, err := srv.SaveTournament(context.Background(), testTournamentID, in)
		require.NoError(t, err)
		require.Len(t, corrected.Results, 1)
		assert.Equal(t, in.Players[2].ID, corrected.Results[0].PlayerID)
		assert.Equal(t, 2, *corrected.Results[0].Place)
		assert.Len(t, first.Results, 2)
		assert.Equal(t, int64(3700), first.Results[0].Prize)
		in.Results = nil
		in.Players[0].Rebuys = nil
		cleared, err := srv.SaveTournament(context.Background(), testTournamentID, in)
		require.NoError(t, err)
		assert.False(t, cleared.HasResults())
		assert.Equal(t, int64(4000), cleared.Pot())
		assert.Empty(t, cleared.Results)
	})
	t.Run("updates prize winners by stable id after rename", func(t *testing.T) {
		in := validSaveTournamentInput()
		repo := &mockTournamentRepository{getTournament: serviceTournamentFixture(t, in)}
		in.Players[0].Name = "Renamed"
		place := 1
		in.Results = []domain.Result{{PlayerID: in.Players[0].ID, Place: &place, Prize: 4000}}
		tr, err := NewTournamentService(repo).SaveTournament(context.Background(), testTournamentID, in)
		require.NoError(t, err)
		require.NotEmpty(t, tr.CalculateTransfers())
		assert.Equal(t, "Renamed", tr.CalculateTransfers()[0].To)
	})
	t.Run("can replace participants and recomputes generated plans", func(t *testing.T) {
		in := validSaveTournamentInput()
		repo := &mockTournamentRepository{getTournament: serviceTournamentFixture(t, in)}
		in.Players = append(in.Players, domain.TournamentPlayer{ID: uuid.New(), Name: "E"})
		in.LevelDuration = 30 * time.Minute
		tr, err := NewTournamentService(repo).SaveTournament(context.Background(), testTournamentID, in)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), tr.Pot())
		assert.Len(t, tr.BlindStructure, 4)
		expected, err := domain.GenerateStackPlan(5, in.ChipSet, in.Style)
		require.NoError(t, err)
		assert.Equal(t, expected, tr.StartingStack)
	})
	tests := []struct {
		name    string
		change  func(*SaveTournamentInput)
		wantErr error
	}{
		{"player id required", func(in *SaveTournamentInput) { in.Players[0].ID = uuid.Nil }, domain.ErrInvalidPlayerID},
		{"rebuy id required", func(in *SaveTournamentInput) { in.Players[0].Rebuys = []domain.Rebuy{{}} }, domain.ErrInvalidRebuyID},
		{"wrong total", func(in *SaveTournamentInput) { in.Results = []domain.Result{{PlayerID: in.Players[0].ID, Prize: 123}} }, domain.ErrPrizeSumMismatch},
		{"unknown winner", func(in *SaveTournamentInput) { in.Results = []domain.Result{{PlayerID: uuid.New(), Prize: 4000}} }, domain.ErrPlayerNotFound},
		{"generation fails", func(in *SaveTournamentInput) { in.Duration = 20 * time.Minute }, domain.ErrNotEnoughLevels},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validSaveTournamentInput()
			original := serviceTournamentFixture(t, in)
			repo := &mockTournamentRepository{getTournament: original}
			tt.change(&in)
			tr, err := NewTournamentService(repo).SaveTournament(context.Background(), testTournamentID, in)
			require.ErrorIs(t, err, app.ErrValidation)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, tr)
			assert.False(t, repo.updateCalled)
			assert.Empty(t, original.Results)
		})
	}
	t.Run("update error does not mutate loaded card", func(t *testing.T) {
		in := validSaveTournamentInput()
		original := serviceTournamentFixture(t, in)
		repoErr := errors.New("update err")
		repo := &mockTournamentRepository{getTournament: original, updateErr: repoErr}
		in.Players[0].Name = "Changed"
		in.Name = "Changed"
		tr, err := NewTournamentService(repo).SaveTournament(context.Background(), testTournamentID, in)
		require.ErrorIs(t, err, repoErr)
		assert.Nil(t, tr)
		assert.Equal(t, 1, repo.updateCount)
		assert.Equal(t, "A", original.Players[0].Name)
		assert.Equal(t, "Friday Game", original.Name)
	})
	t.Run("get error avoids update", func(t *testing.T) {
		repo := &mockTournamentRepository{getErr: app.ErrTournamentNotFound}
		tr, err := NewTournamentService(repo).SaveTournament(context.Background(), testTournamentID, validSaveTournamentInput())
		require.ErrorIs(t, err, app.ErrTournamentNotFound)
		assert.Nil(t, tr)
		assert.False(t, repo.updateCalled)
	})
	t.Run("invalid id avoids repository", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		tr, err := NewTournamentService(repo).SaveTournament(context.Background(), uuid.Nil, validSaveTournamentInput())
		require.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, tr)
		assert.False(t, repo.getCalled)
	})
}

func TestTournamentService_GetTournamentByID(t *testing.T) {
	t.Run("read has no writes", func(t *testing.T) {
		want := &domain.Tournament{ID: testTournamentID}
		repo := &mockTournamentRepository{getTournament: want}
		got, err := NewTournamentService(repo).GetTournamentByID(context.Background(), testTournamentID)
		require.NoError(t, err)
		assert.Same(t, want, got)
		assert.Equal(t, testTournamentID, repo.getID)
		assert.False(t, repo.updateCalled)
	})
	t.Run("invalid id", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		got, err := NewTournamentService(repo).GetTournamentByID(context.Background(), uuid.Nil)
		require.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, got)
		assert.False(t, repo.getCalled)
	})
	t.Run("repo fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		got, err := NewTournamentService(repo).GetTournamentByID(context.Background(), testTournamentID)
		require.ErrorIs(t, err, repoErr)
		assert.Nil(t, got)
	})
}
func TestTournamentService_DeleteTournament(t *testing.T) {
	t.Run("deletes", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		require.NoError(t, NewTournamentService(repo).DeleteTournament(context.Background(), testTournamentID))
		assert.Equal(t, testTournamentID, repo.deleteID)
	})
	t.Run("invalid id", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		require.ErrorIs(t, NewTournamentService(repo).DeleteTournament(context.Background(), uuid.Nil), app.ErrInvalidTournamentID)
		assert.False(t, repo.deleteCalled)
	})
	t.Run("repo fails", func(t *testing.T) {
		repo := &mockTournamentRepository{deleteErr: app.ErrTournamentNotFound}
		require.ErrorIs(t, NewTournamentService(repo).DeleteTournament(context.Background(), testTournamentID), app.ErrTournamentNotFound)
	})
}
func TestTournamentService_ListTournaments(t *testing.T) {
	t.Run("passes pagination", func(t *testing.T) {
		want := []*domain.Tournament{{ID: testTournamentID}}
		repo := &mockTournamentRepository{tournaments: want}
		got, err := NewTournamentService(repo).ListTournaments(context.Background(), 20, 10)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, 20, repo.limit)
		assert.Equal(t, 10, repo.offset)
	})
	for _, args := range [][2]int{{0, 0}, {101, 0}, {-1, 0}, {10, -1}} {
		t.Run("invalid pagination", func(t *testing.T) {
			repo := &mockTournamentRepository{}
			got, err := NewTournamentService(repo).ListTournaments(context.Background(), args[0], args[1])
			require.ErrorIs(t, err, app.ErrValidation)
			assert.Nil(t, got)
			assert.False(t, repo.listCalled)
		})
	}
	t.Run("repo fails", func(t *testing.T) {
		repoErr := errors.New("list err")
		repo := &mockTournamentRepository{listErr: repoErr}
		got, err := NewTournamentService(repo).ListTournaments(context.Background(), 50, 0)
		require.ErrorIs(t, err, repoErr)
		assert.Nil(t, got)
	})
}
