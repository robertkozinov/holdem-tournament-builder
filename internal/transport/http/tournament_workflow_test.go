package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The real HTTP and service layers use a snapshot repository without external services.
type memoryTournamentRepository struct {
	cards  map[uuid.UUID][]byte
	owners map[uuid.UUID]uuid.UUID
}

func (r *memoryTournamentRepository) Create(ctx context.Context, ownerID uuid.UUID, tr *domain.Tournament) (uuid.UUID, error) {
	if r.owners == nil {
		r.owners = make(map[uuid.UUID]uuid.UUID)
	}
	tr.ID = uuid.New()
	r.owners[tr.ID] = ownerID
	return tr.ID, r.Update(ctx, ownerID, tr)
}

func (r *memoryTournamentRepository) GetByID(ctx context.Context, ownerID, id uuid.UUID) (*domain.Tournament, error) {
	data, ok := r.cards[id]
	if !ok || ownerID == uuid.Nil || r.owners[id] != ownerID {
		return nil, app.ErrTournamentNotFound
	}
	var tr domain.Tournament
	err := json.Unmarshal(data, &tr)
	return &tr, err
}

func (r *memoryTournamentRepository) Update(ctx context.Context, ownerID uuid.UUID, tr *domain.Tournament) error {
	if ownerID == uuid.Nil || r.owners[tr.ID] != ownerID {
		return app.ErrTournamentNotFound
	}
	data, err := json.Marshal(tr)
	if err == nil {
		r.cards[tr.ID] = data
	}
	return err
}

func (r *memoryTournamentRepository) UpdateLocked(ctx context.Context, ownerID, id uuid.UUID, change func(*domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error) {
	original, err := r.GetByID(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	updated, err := change(original)
	if err != nil {
		return nil, err
	}
	if err := r.Update(ctx, ownerID, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *memoryTournamentRepository) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	if _, err := r.GetByID(ctx, ownerID, id); err != nil {
		return app.ErrTournamentNotFound
	}
	delete(r.cards, id)
	delete(r.owners, id)
	return nil
}

func (r *memoryTournamentRepository) List(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.Tournament, error) {
	all := make([]*domain.Tournament, 0, len(r.cards))
	for id := range r.cards {
		if r.owners[id] != ownerID {
			continue
		}
		tr, err := r.GetByID(ctx, ownerID, id)
		if err != nil {
			return nil, err
		}
		all = append(all, tr)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Date.After(all[j].Date) })
	if offset >= len(all) {
		return []*domain.Tournament{}, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

func TestTournamentHTTPWorkflow(t *testing.T) {
	repo := &memoryTournamentRepository{cards: make(map[uuid.UUID][]byte)}
	router := newTournamentTestRouter(NewTournamentHandler(service.NewTournamentService(repo)))
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		require.Equal(t, status, rec.Code, rec.Body.String())
		return rec
	}
	read := func(path string) TournamentResponse {
		t.Helper()
		rec := request(http.MethodGet, path, "", http.StatusOK)
		var card TournamentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &card))
		return card
	}
	create := request(http.MethodPost, "/tournaments", validCreateTournamentJSON(), http.StatusCreated)
	var created CreateTournamentResponse
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
	path := "/tournaments/" + created.ID
	initial := read(path)
	assert.False(t, initial.HasResults)
	assert.Empty(t, initial.Transfers)
	assert.Equal(t, int64(2000), initial.Pot)
	assert.NotEmpty(t, initial.StartingStack.Distribution)
	assert.Len(t, initial.BlindStructure, 6)
	settings := settingsRequestFixture()
	settings.Players = []PlayerRequest{
		{ID: uuid.MustParse(initial.Players[0].ID), Name: "A", Rebuys: []RebuyRequest{{ID: uuid.New(), Paid: false}}},
		{ID: uuid.MustParse(initial.Players[1].ID), Name: "B", BuyInPaid: true},
	}
	settings.ChipSet = []ChipRequest{{Value: 25, Count: 80}, {Value: 100, Count: 80}}
	req := SaveTournamentRequest{TournamentSettingsRequest: settings, Results: []ResultRequest{
		{PlayerID: settings.Players[0].ID, IsSplit: true, Prize: 2500},
		{PlayerID: settings.Players[1].ID, IsSplit: true, Prize: 500},
	}}
	save := func() TournamentResponse {
		t.Helper()
		payload, err := json.Marshal(req)
		require.NoError(t, err)
		rec := request(http.MethodPut, path, string(payload), http.StatusOK)
		var card TournamentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &card))
		return card
	}
	first := save()
	assert.True(t, first.HasResults)
	assert.Equal(t, int64(3000), first.Pot)
	assert.Equal(t, int64(3000), first.SuggestedPayouts[0].Value)
	assert.Equal(t, int64(2500), first.Results[0].Prize)
	assert.Nil(t, first.Results[0].Place)
	require.Len(t, first.Transfers, 1)
	assert.Equal(t, TransferResponse{From: "B", To: "A", Amount: 500}, first.Transfers[0])
	assert.Equal(t, first, save())
	assert.Equal(t, first, read(path))
	assert.Len(t, repo.cards, 1)
	place := 1
	req.Results = []ResultRequest{{PlayerID: settings.Players[1].ID, Place: &place, Prize: 3000}}
	corrected := save()
	require.Len(t, corrected.Results, 1)
	assert.Equal(t, "B", corrected.Results[0].Name)
	assert.Equal(t, int64(1000), corrected.BuyInAmount)
	assert.Equal(t, TransferResponse{From: "A", To: "B", Amount: 2000}, corrected.Transfers[0])
	req.SettlementMode = "bank"
	bank := save()
	assert.Empty(t, bank.Transfers)
	assert.False(t, bank.Players[0].Rebuys[0].Paid)
	assert.True(t, bank.Players[1].BuyInPaid)
	assert.Equal(t, int64(3000), bank.Pot)
	req.Results[0].Prize = 2999
	bad, err := json.Marshal(req)
	require.NoError(t, err)
	request(http.MethodPut, path, string(bad), http.StatusBadRequest)
	assert.Equal(t, bank, read(path))
	req.Results = nil
	req.Players[0].Rebuys = nil
	req.SettlementMode = "p2p"
	draft := save()
	assert.False(t, draft.HasResults)
	assert.Empty(t, draft.Results)
	assert.Empty(t, draft.Transfers)
	assert.Equal(t, int64(2000), draft.Pot)
	listed := request(http.MethodGet, "/tournaments", "", http.StatusOK)
	var cards []TournamentResponse
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &cards))
	require.Len(t, cards, 1)
	assert.Equal(t, draft, cards[0])
	request(http.MethodDelete, path, "", http.StatusNoContent)
	request(http.MethodGet, path, "", http.StatusNotFound)
	assert.JSONEq(t, `[]`, request(http.MethodGet, "/tournaments", "", http.StatusOK).Body.String())
}

func TestTournamentHTTPPlayerPatchRegression(t *testing.T) {
	repo := &memoryTournamentRepository{cards: make(map[uuid.UUID][]byte)}
	router := newTournamentTestRouter(NewTournamentHandler(service.NewTournamentService(repo)))
	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(validCreateTournamentJSON())))
	require.Equal(t, http.StatusCreated, create.Code, create.Body.String())
	var created CreateTournamentResponse
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
	id := uuid.MustParse(created.ID)
	before, err := repo.GetByID(context.Background(), testUserID, id)
	require.NoError(t, err)
	path := "/tournaments/" + created.ID + "/players/" + before.Players[0].ID.String()
	patch := httptest.NewRecorder()
	router.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"buy_in_paid":true}`)))
	require.Equal(t, http.StatusOK, patch.Code, patch.Body.String())
	after, err := repo.GetByID(context.Background(), testUserID, id)
	require.NoError(t, err)
	before.Players[0].BuyInPaid = true
	assert.Equal(t, before, after)
	expected, err := newTournamentResponse(after)
	require.NoError(t, err)
	var actual TournamentResponse
	require.NoError(t, json.Unmarshal(patch.Body.Bytes(), &actual))
	assert.Equal(t, expected, actual)
}

func TestTournamentHTTPPlayerPatchPreservesValidationErrorMessage(t *testing.T) {
	f := newPlayerPatchHTTPFixture(t, domain.SettlementBank)
	before := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
	payload, err := json.Marshal(map[string]any{
		"buy_in_paid": true,
		"rebuys":      []RebuyRequest{{ID: uuid.Nil, Paid: false}},
	})
	require.NoError(t, err)
	rec := f.request(t, http.MethodPatch, f.path, string(payload), http.StatusBadRequest)
	expected, err := json.Marshal(map[string]string{
		"error": app.ErrValidation.Error() + ": patch tournament player: " + domain.ErrInvalidRebuyID.Error(),
	})
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), rec.Body.String())
	assert.Equal(t, before, f.repo.cards[f.tournament.ID])
	assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
}

func TestTournamentHTTPPlayerPatchWorkflow(t *testing.T) {
	for _, mode := range []domain.SettlementMode{domain.SettlementBank, domain.SettlementP2P} {
		t.Run(string(mode), func(t *testing.T) {
			f := newPlayerPatchHTTPFixture(t, mode)
			initialPot := f.tournament.Pot()
			f.tournament.Players[0].BuyInPaid = false
			unpaid := f.patch(t, `{"buy_in_paid":false}`)
			assert.Equal(t, initialPot, unpaid.Pot)
			f.tournament.Players[0].BuyInPaid = true
			paid := f.patch(t, `{"buy_in_paid":true,"rebuys":null}`)
			assert.Equal(t, initialPot, paid.Pot)
			f.tournament.Players[0].Rebuys[0].Paid = true
			rebuys := []RebuyRequest{{ID: f.tournament.Players[0].Rebuys[0].ID, Paid: true}}
			payload, err := json.Marshal(map[string]any{"buy_in_paid": nil, "rebuys": rebuys})
			require.NoError(t, err)
			rebuyPaid := f.patch(t, string(payload))
			assert.Equal(t, initialPot, rebuyPaid.Pot)
			newRebuy := domain.Rebuy{ID: uuid.New(), Paid: false}
			f.tournament.Players[0].Rebuys = append(f.tournament.Players[0].Rebuys, newRebuy)
			rebuys = append(rebuys, RebuyRequest{ID: newRebuy.ID, Paid: false})
			payload, err = json.Marshal(map[string]any{"rebuys": rebuys})
			require.NoError(t, err)
			added := f.patch(t, string(payload))
			assert.Equal(t, initialPot+f.tournament.BuyInAmount, added.Pot)
			snapshot := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
			assert.Equal(t, added, f.patch(t, string(payload)))
			assert.Equal(t, snapshot, f.repo.cards[f.tournament.ID])
			f.tournament.Players[0].Rebuys = []domain.Rebuy{newRebuy}
			payload, err = json.Marshal(map[string]any{"rebuys": []RebuyRequest{{ID: newRebuy.ID, Paid: false}}})
			require.NoError(t, err)
			removed := f.patch(t, string(payload))
			assert.Equal(t, initialPot, removed.Pot)
			f.tournament.Players[0].Rebuys = []domain.Rebuy{}
			cleared := f.patch(t, `{"rebuys":[]}`)
			assert.Equal(t, initialPot-f.tournament.BuyInAmount, cleared.Pot)
			assert.Empty(t, cleared.Players[0].Rebuys)
			assert.NotNil(t, cleared.Players[0].Rebuys)
			assert.Equal(t, cleared, f.patch(t, `{"rebuys":[]}`))
			f.tournament.Players[0].BuyInPaid = false
			f.patch(t, `{"buy_in_paid":false,"rebuys":[]}`)
		})
	}
}

func TestTournamentHTTPRejectsDuplicateChipValues(t *testing.T) {
	manyChips := make([]ChipRequest, 800)
	for i := range manyChips {
		manyChips[i] = ChipRequest{Value: 25000000000000, Count: 4}
	}
	tests := []struct {
		name    string
		players []PlayerRequest
		chips   []ChipRequest
	}{
		{
			name:    "two identical rows for four players",
			players: []PlayerRequest{{ID: uuid.New(), Name: "A"}, {ID: uuid.New(), Name: "B"}, {ID: uuid.New(), Name: "C"}, {ID: uuid.New(), Name: "D"}},
			chips:   []ChipRequest{{Value: 500, Count: 40}, {Value: 500, Count: 40}},
		},
		{
			name:    "many rows for two players",
			players: []PlayerRequest{{ID: uuid.New(), Name: "A"}, {ID: uuid.New(), Name: "B"}},
			chips:   manyChips,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, method := range []string{http.MethodPost, http.MethodPut} {
				t.Run(method, func(t *testing.T) {
					repo := &memoryTournamentRepository{cards: make(map[uuid.UUID][]byte)}
					router := newTournamentTestRouter(NewTournamentHandler(service.NewTournamentService(repo)))
					request := func(method, path, body string) *httptest.ResponseRecorder {
						t.Helper()
						rec := httptest.NewRecorder()
						router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
						return rec
					}
					settings := settingsRequestFixture()
					settings.Players = tt.players
					if method == http.MethodPost {
						settings.ChipSet = tt.chips
						payload, err := json.Marshal(CreateTournamentRequest{TournamentSettingsRequest: settings, BuyInAmount: 1000})
						require.NoError(t, err)
						rec := request(method, "/tournaments", string(payload))
						require.Equal(t, http.StatusBadRequest, rec.Code)
						assert.Contains(t, rec.Body.String(), domain.ErrDuplicateChipValue.Error())
						assert.Empty(t, repo.cards)
						return
					}

					payload, err := json.Marshal(CreateTournamentRequest{TournamentSettingsRequest: settings, BuyInAmount: 1000})
					require.NoError(t, err)
					create := request(http.MethodPost, "/tournaments", string(payload))
					require.Equal(t, http.StatusCreated, create.Code, create.Body.String())
					var created CreateTournamentResponse
					require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
					id := uuid.MustParse(created.ID)
					previous := append([]byte(nil), repo.cards[id]...)
					path := "/tournaments/" + created.ID
					before := request(http.MethodGet, path, "")
					require.Equal(t, http.StatusOK, before.Code, before.Body.String())

					settings.Name = "Changed Game"
					settings.ChipSet = tt.chips
					payload, err = json.Marshal(SaveTournamentRequest{TournamentSettingsRequest: settings})
					require.NoError(t, err)
					rec := request(method, path, string(payload))
					require.Equal(t, http.StatusBadRequest, rec.Code)
					assert.Contains(t, rec.Body.String(), domain.ErrDuplicateChipValue.Error())
					assert.Len(t, repo.cards, 1)
					assert.Equal(t, previous, repo.cards[id])
					after := request(http.MethodGet, path, "")
					require.Equal(t, http.StatusOK, after.Code, after.Body.String())
					assert.JSONEq(t, before.Body.String(), after.Body.String())
				})
			}
		})
	}
}
