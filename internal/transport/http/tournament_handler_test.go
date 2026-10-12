package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockTournamentService struct {
	createCalled      bool
	createInput       service.CreateTournamentInput
	createID          uuid.UUID
	createErr         error
	getCalled         bool
	getID             uuid.UUID
	tournament        *domain.Tournament
	getErr            error
	saveCalled        bool
	saveID            uuid.UUID
	saveInput         service.SaveTournamentInput
	saveErr           error
	patchCalled       bool
	patchTournamentID uuid.UUID
	patchPlayerID     uuid.UUID
	patchInput        service.PatchTournamentPlayerInput
	patchErr          error
	deleteCalled      bool
	deleteID          uuid.UUID
	deleteErr         error
	listCalled        bool
	limit, offset     int
	tournaments       []*domain.Tournament
	listErr           error
}

func (s *mockTournamentService) CreateTournament(ctx context.Context, in service.CreateTournamentInput) (uuid.UUID, error) {
	s.createCalled = true
	s.createInput = in
	return s.createID, s.createErr
}
func (s *mockTournamentService) GetTournamentByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	s.getCalled = true
	s.getID = id
	return s.tournament, s.getErr
}
func (s *mockTournamentService) SaveTournament(ctx context.Context, id uuid.UUID, in service.SaveTournamentInput) (*domain.Tournament, error) {
	s.saveCalled = true
	s.saveID = id
	s.saveInput = in
	return s.tournament, s.saveErr
}
func (s *mockTournamentService) PatchTournamentPlayer(ctx context.Context, tournamentID, playerID uuid.UUID, in service.PatchTournamentPlayerInput) (*domain.Tournament, error) {
	s.patchCalled = true
	s.patchTournamentID = tournamentID
	s.patchPlayerID = playerID
	s.patchInput = in
	return s.tournament, s.patchErr
}
func (s *mockTournamentService) DeleteTournament(ctx context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.deleteID = id
	return s.deleteErr
}
func (s *mockTournamentService) ListTournaments(ctx context.Context, limit, offset int) ([]*domain.Tournament, error) {
	s.listCalled = true
	s.limit = limit
	s.offset = offset
	return s.tournaments, s.listErr
}

func validCreateTournamentJSON() string {
	return `{"name":"Friday Game","date":"2026-10-07T20:00:00Z","players":[{"name":"A"},{"name":"B"}],"buy_in_amount":1000,"settlement_mode":"p2p","chip_set":[{"value":25,"count":80},{"value":100,"count":80}],"style":"standard","duration_minutes":120,"level_duration_minutes":20}`
}
func handlerTournamentFixture() *domain.Tournament {
	a, b := uuid.New(), uuid.New()
	place := 1
	return &domain.Tournament{ID: uuid.New(), Name: "Friday Game", Date: time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC),
		Players:     []domain.TournamentPlayer{{ID: a, Name: "A", BuyInPaid: true, Rebuys: []domain.Rebuy{{ID: uuid.New(), Paid: false}}}, {ID: b, Name: "B"}},
		BuyInAmount: 1000, SettlementMode: domain.SettlementP2P, Style: domain.StyleStandard,
		ChipSet: []domain.ChipDenomination{{Value: 25, Count: 80}}, Duration: 2 * time.Hour, LevelDuration: 20 * time.Minute,
		StartingStack:  domain.StackPlan{Distribution: []domain.ChipDenomination{{Value: 25, Count: 10}}},
		BlindStructure: []domain.BlindLevel{{SmallBlind: 25, BigBlind: 50, Duration: 20 * time.Minute}},
		Results:        []domain.Result{{PlayerID: a, Place: &place, Prize: 3000}},
	}
}
func serve(t *testing.T, srv *mockTournamentService, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newTournamentTestRouter(NewTournamentHandler(srv)).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestTournamentHandler_CreateTournament(t *testing.T) {
	t.Run("creates draft", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{createID: id}
		rec := serve(t, srv, http.MethodPost, "/tournaments", validCreateTournamentJSON())
		require.Equal(t, http.StatusCreated, rec.Code)
		assert.True(t, srv.createCalled)
		assert.Equal(t, "Friday Game", srv.createInput.Name)
		assert.Len(t, srv.createInput.Players, 2)
		assert.Equal(t, domain.SettlementP2P, srv.createInput.SettlementMode)
		var body CreateTournamentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, id.String(), body.ID)
	})
	for _, body := range []string{
		"", `{`, `null`, validCreateTournamentJSON() + `{}`,
		strings.Replace(validCreateTournamentJSON(), `"style":"standard"`, `"style":"unknown"`, 1),
		strings.Replace(validCreateTournamentJSON(), `"settlement_mode":"p2p"`, `"settlement_mode":"unknown"`, 1),
		strings.Replace(validCreateTournamentJSON(), `"name":"Friday Game"`, `"name":"Friday Game","payout":{"payout_mode":"custom"}`, 1),
		strings.Replace(validCreateTournamentJSON(), `"name":"A"`, `"name":"A","id":"invalid"`, 1),
	} {
		t.Run("rejects malformed request", func(t *testing.T) {
			srv := &mockTournamentService{}
			rec := serve(t, srv, http.MethodPost, "/tournaments", body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, srv.createCalled)
		})
	}
	t.Run("rejects oversized body", func(t *testing.T) {
		srv := &mockTournamentService{}
		body := strings.Replace(validCreateTournamentJSON(), "Friday Game", strings.Repeat("a", 1<<20), 1)
		rec := serve(t, srv, http.MethodPost, "/tournaments", body)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		assert.False(t, srv.createCalled)
	})
	for _, tt := range []struct {
		err    error
		status int
	}{{fmtValidationError(), 400}, {errors.New("db secret"), 500}} {
		t.Run("service error", func(t *testing.T) {
			rec := serve(t, &mockTournamentService{createErr: tt.err}, http.MethodPost, "/tournaments", validCreateTournamentJSON())
			assert.Equal(t, tt.status, rec.Code)
			assert.NotContains(t, rec.Body.String(), "db secret")
		})
	}
}
func fmtValidationError() error { return errors.Join(app.ErrValidation, domain.ErrEmptyName) }

func TestTournamentHandler_ChipSetContract(t *testing.T) {
	tr := handlerTournamentFixture()
	for _, tt := range []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{"create", http.MethodPost, "/tournaments", validCreateTournamentJSON(), http.StatusCreated},
		{"save", http.MethodPut, "/tournaments/" + tr.ID.String(), strings.Replace(validCreateTournamentJSON(), `"buy_in_amount":1000,`, "", 1), http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := &mockTournamentService{createID: tr.ID, tournament: tr}
			rec := serve(t, srv, tt.method, tt.path, tt.body)
			require.Equal(t, tt.status, rec.Code)
			expected := []domain.ChipDenomination{{Value: 25, Count: 80}, {Value: 100, Count: 80}}
			if tt.method == http.MethodPost {
				assert.True(t, srv.createCalled)
				assert.Equal(t, expected, srv.createInput.ChipSet)
			} else {
				assert.True(t, srv.saveCalled)
				assert.Equal(t, expected, srv.saveInput.ChipSet)
			}
		})
		t.Run(tt.name+" rejects legacy chips field", func(t *testing.T) {
			srv := &mockTournamentService{createID: tr.ID, tournament: tr}
			body := strings.Replace(tt.body, `"chip_set":`, `"chips":`, 1)
			rec := serve(t, srv, tt.method, tt.path, body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, srv.createCalled)
			assert.False(t, srv.saveCalled)
		})
	}
}

func TestTournamentHandler_GetTournamentByID(t *testing.T) {
	t.Run("returns complete card and derived values", func(t *testing.T) {
		tr := handlerTournamentFixture()
		srv := &mockTournamentService{tournament: tr}
		rec := serve(t, srv, http.MethodGet, "/tournaments/"+tr.ID.String(), "")
		require.Equal(t, 200, rec.Code)
		assert.Equal(t, tr.ID, srv.getID)
		var body TournamentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, tr.ID.String(), body.ID)
		assert.Equal(t, 120, body.DurationMinutes)
		assert.Equal(t, 20, body.LevelDurationMinutes)
		assert.Equal(t, int64(3000), body.Pot)
		assert.Equal(t, int64(250), body.StartingStack.Total)
		assert.True(t, body.HasResults)
		require.Len(t, body.Results, 1)
		assert.Equal(t, 1, *body.Results[0].Place)
		assert.False(t, body.Results[0].IsSplit)
		require.Len(t, body.Players, 2)
		assert.True(t, body.Players[0].BuyInPaid)
		assert.False(t, body.Players[0].Rebuys[0].Paid)
		require.Len(t, body.SuggestedPayouts, 1)
		assert.Equal(t, int64(3000), body.SuggestedPayouts[0].Value)
		require.Len(t, body.Transfers, 1)
		assert.Equal(t, "B", body.Transfers[0].From)
		assert.Equal(t, "A", body.Transfers[0].To)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fields))
		assert.Contains(t, fields, "chip_set")
		assert.NotContains(t, fields, "chips")
		assert.NotContains(t, fields, "status")
		assert.NotContains(t, fields, "payout_spots")
	})
	t.Run("bank split and draft return empty transfer arrays", func(t *testing.T) {
		tr := handlerTournamentFixture()
		tr.SettlementMode = domain.SettlementBank
		tr.Results[0].Place = nil
		rec := serve(t, &mockTournamentService{tournament: tr}, http.MethodGet, "/tournaments/"+tr.ID.String(), "")
		require.Equal(t, 200, rec.Code)
		var body TournamentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.True(t, body.Results[0].IsSplit)
		assert.Nil(t, body.Results[0].Place)
		assert.JSONEq(t, `[]`, jsonField(t, rec, "transfers"))
		tr.Results = nil
		rec = serve(t, &mockTournamentService{tournament: tr}, http.MethodGet, "/tournaments/"+tr.ID.String(), "")
		require.Equal(t, 200, rec.Code)
		assert.JSONEq(t, `[]`, jsonField(t, rec, "results"))
		assert.JSONEq(t, `false`, jsonField(t, rec, "has_results"))
	})
}
func jsonField(t *testing.T, rec *httptest.ResponseRecorder, field string) string {
	t.Helper()
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return string(body[field])
}

func TestTournamentHandler_SaveTournament(t *testing.T) {
	tr := handlerTournamentFixture()
	settings := settingsRequestFixture()
	settings.Players = []PlayerRequest{{ID: tr.Players[0].ID, Name: "A"}, {ID: tr.Players[1].ID, Name: "B"}}
	place := 1
	req := SaveTournamentRequest{TournamentSettingsRequest: settings, Results: []ResultRequest{{PlayerID: tr.Players[0].ID, Place: &place, Prize: 2000}}}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	path := "/tournaments/" + tr.ID.String()
	t.Run("saves complete card", func(t *testing.T) {
		srv := &mockTournamentService{tournament: tr}
		rec := serve(t, srv, http.MethodPut, path, string(data))
		require.Equal(t, 200, rec.Code)
		assert.True(t, srv.saveCalled)
		assert.Equal(t, tr.ID, srv.saveID)
		require.Len(t, srv.saveInput.Results, 1)
		assert.Equal(t, int64(2000), srv.saveInput.Results[0].Prize)
		assert.Equal(t, tr.ID.String(), strings.Trim(jsonField(t, rec, "id"), "\""))
	})
	t.Run("cannot change buy in", func(t *testing.T) {
		srv := &mockTournamentService{}
		body := strings.Replace(string(data), `"name":`, `"buy_in_amount":99,"name":`, 1)
		rec := serve(t, srv, http.MethodPut, path, body)
		assert.Equal(t, 400, rec.Code)
		assert.False(t, srv.saveCalled)
	})
	t.Run("requires explicit split", func(t *testing.T) {
		req.Results[0].Place = nil
		bad, err := json.Marshal(req)
		require.NoError(t, err)
		srv := &mockTournamentService{}
		rec := serve(t, srv, http.MethodPut, path, string(bad))
		assert.Equal(t, 400, rec.Code)
		assert.False(t, srv.saveCalled)
	})
}
func TestTournamentHandler_IDAndServiceErrors(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method+" invalid id", func(t *testing.T) {
			srv := &mockTournamentService{}
			rec := serve(t, srv, method, "/tournaments/not-id", validCreateTournamentJSON())
			assert.Equal(t, 400, rec.Code)
			assert.False(t, srv.getCalled)
			assert.False(t, srv.saveCalled)
			assert.False(t, srv.deleteCalled)
		})
		for _, tt := range []struct {
			err    error
			status int
		}{{app.ErrTournamentNotFound, 404}, {errors.Join(app.ErrValidation, domain.ErrPlayerNotFound), 400}, {errors.New("private error"), 500}} {
			t.Run(method+" service error", func(t *testing.T) {
				srv := &mockTournamentService{getErr: tt.err, saveErr: tt.err, deleteErr: tt.err}
				payload := strings.Replace(validCreateTournamentJSON(), `"buy_in_amount":1000,`, "", 1)
				rec := serve(t, srv, method, "/tournaments/"+uuid.New().String(), payload)
				assert.Equal(t, tt.status, rec.Code)
				assert.NotContains(t, rec.Body.String(), "private error")
			})
		}
	}
	t.Run("deletes without body", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		rec := serve(t, srv, http.MethodDelete, "/tournaments/"+id.String(), "")
		assert.Equal(t, 204, rec.Code)
		assert.Equal(t, id, srv.deleteID)
		assert.Empty(t, rec.Body.String())
	})
}
func TestTournamentHandler_ListTournaments(t *testing.T) {
	t.Run("list defaults and empty array", func(t *testing.T) {
		srv := &mockTournamentService{}
		rec := serve(t, srv, http.MethodGet, "/tournaments", "")
		require.Equal(t, 200, rec.Code)
		assert.Equal(t, 50, srv.limit)
		assert.Zero(t, srv.offset)
		assert.JSONEq(t, `[]`, rec.Body.String())
	})
	t.Run("maps cards and forwards pagination", func(t *testing.T) {
		tr := handlerTournamentFixture()
		srv := &mockTournamentService{tournaments: []*domain.Tournament{tr}}
		rec := serve(t, srv, http.MethodGet, "/tournaments?limit=20&offset=5", "")
		require.Equal(t, 200, rec.Code)
		assert.Equal(t, 20, srv.limit)
		assert.Equal(t, 5, srv.offset)
		var cards []TournamentResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cards))
		require.Len(t, cards, 1)
		assert.Equal(t, tr.ID.String(), cards[0].ID)
	})
	for _, query := range []string{"limit=no", "limit=", "limit=0", "limit=101", "limit=-1", "offset=-1", "offset=x", "offset=", "limit=2&limit=3", "offset=1&offset=2"} {
		t.Run(query, func(t *testing.T) {
			srv := &mockTournamentService{}
			rec := serve(t, srv, http.MethodGet, "/tournaments?"+query, "")
			assert.Equal(t, 400, rec.Code)
			assert.False(t, srv.listCalled)
		})
	}
	t.Run("repo error hidden", func(t *testing.T) {
		rec := serve(t, &mockTournamentService{listErr: errors.New("secret")}, http.MethodGet, "/tournaments", "")
		assert.Equal(t, 500, rec.Code)
		assert.NotContains(t, rec.Body.String(), "secret")
	})
}
