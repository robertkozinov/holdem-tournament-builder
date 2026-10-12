package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type playerPatchHTTPFixture struct {
	router        http.Handler
	repo          *memoryTournamentRepository
	tournament    *domain.Tournament
	path          string
	otherID       uuid.UUID
	otherSnapshot []byte
}

func newPlayerPatchHTTPFixture(t *testing.T, mode domain.SettlementMode) *playerPatchHTTPFixture {
	t.Helper()
	repo := &memoryTournamentRepository{cards: make(map[uuid.UUID][]byte)}
	f := &playerPatchHTTPFixture{repo: repo, router: newTournamentTestRouter(NewTournamentHandler(service.NewTournamentService(repo)))}
	settings := settingsRequestFixture()
	settings.SettlementMode = string(mode)
	settings.ChipSet = []ChipRequest{{Value: 25, Count: 80}, {Value: 100, Count: 80}}
	settings.Players[1].BuyInPaid = true
	settings.Players[1].Rebuys = []RebuyRequest{{ID: uuid.New(), Paid: true}}
	payload, err := json.Marshal(CreateTournamentRequest{TournamentSettingsRequest: settings, BuyInAmount: 1000})
	require.NoError(t, err)
	create := f.request(t, http.MethodPost, "/tournaments", string(payload), http.StatusCreated)
	var created CreateTournamentResponse
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
	f.tournament, err = repo.GetByID(context.Background(), testUserID, uuid.MustParse(created.ID))
	require.NoError(t, err)
	f.path = "/tournaments/" + created.ID + "/players/" + f.tournament.Players[0].ID.String()
	other := f.request(t, http.MethodPost, "/tournaments", validCreateTournamentJSON(), http.StatusCreated)
	require.NoError(t, json.Unmarshal(other.Body.Bytes(), &created))
	f.otherID = uuid.MustParse(created.ID)
	f.otherSnapshot = append([]byte(nil), repo.cards[f.otherID]...)
	return f
}

func (f *playerPatchHTTPFixture) request(t *testing.T, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	require.Equal(t, status, rec.Code, rec.Body.String())
	return rec
}

func (f *playerPatchHTTPFixture) patch(t *testing.T, body string) TournamentResponse {
	t.Helper()
	rec := f.request(t, http.MethodPatch, f.path, body, http.StatusOK)
	saved, err := f.repo.GetByID(context.Background(), testUserID, f.tournament.ID)
	require.NoError(t, err)
	assert.Equal(t, f.tournament, saved)
	expected, err := newTournamentResponse(f.tournament)
	require.NoError(t, err)
	var actual TournamentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &actual))
	assert.Equal(t, expected, actual)
	get := f.request(t, http.MethodGet, "/tournaments/"+f.tournament.ID.String(), "", http.StatusOK)
	assert.JSONEq(t, rec.Body.String(), get.Body.String())
	assert.Len(t, f.repo.cards, 2)
	assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
	return actual
}

func TestTournamentHTTPPlayerPatchRejectsMissingRebuyPaymentWithoutMutation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		paidField string
	}{
		{name: "null paid", paidField: `,"paid":null`},
		{name: "missing paid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newPlayerPatchHTTPFixture(t, domain.SettlementBank)
			player := f.tournament.Players[1]
			require.True(t, player.Rebuys[0].Paid)
			path := "/tournaments/" + f.tournament.ID.String() + "/players/" + player.ID.String()
			body := fmt.Sprintf(`{"buy_in_paid":false,"rebuys":[{"id":%q%s}]}`, player.Rebuys[0].ID.String(), tt.paidField)
			before := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
			rec := httptest.NewRecorder()
			f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body)))
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.True(t, bytes.Equal(before, f.repo.cards[f.tournament.ID]), "tournament snapshot changed")
			assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
			assert.Len(t, f.repo.cards, 2)
			saved, err := f.repo.GetByID(context.Background(), testUserID, f.tournament.ID)
			require.NoError(t, err)
			require.Len(t, saved.Players[1].Rebuys, 1)
			assert.True(t, saved.Players[1].Rebuys[0].Paid, "existing rebuy payment was cleared")

			srv := &mockTournamentService{tournament: f.tournament}
			mockRec := serve(t, srv, http.MethodPatch, path, body)
			assert.Equal(t, http.StatusBadRequest, mockRec.Code, mockRec.Body.String())
			assert.False(t, srv.patchCalled, "invalid rebuy payment reached the service")
		})
	}
}

func TestTournamentHTTPPlayerPatchRejectsInvalidRequests(t *testing.T) {
	validRebuy := uuid.New().String()
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"empty object", `{}`, http.StatusBadRequest},
		{"null fields only", `{"buy_in_paid":null,"rebuys":null}`, http.StatusBadRequest},
		{"null payment only", `{"buy_in_paid":null}`, http.StatusBadRequest},
		{"null rebuys only", `{"rebuys":null}`, http.StatusBadRequest},
		{"empty body", "", http.StatusBadRequest},
		{"malformed JSON", `{`, http.StatusBadRequest},
		{"null body", `null`, http.StatusBadRequest},
		{"array body", `[]`, http.StatusBadRequest},
		{"trailing object", `{"buy_in_paid":true}{}`, http.StatusBadRequest},
		{"trailing junk", `{"buy_in_paid":true}x`, http.StatusBadRequest},
		{"string payment", `{"buy_in_paid":"false"}`, http.StatusBadRequest},
		{"numeric payment", `{"buy_in_paid":1}`, http.StatusBadRequest},
		{"object rebuys", `{"rebuys":{}}`, http.StatusBadRequest},
		{"boolean rebuys", `{"rebuys":false}`, http.StatusBadRequest},
		{"unknown field", `{"buy_in_paid":true,"name":"Changed"}`, http.StatusBadRequest},
		{"immutable buy in", `{"buy_in_paid":true,"buy_in_amount":2000}`, http.StatusBadRequest},
		{"unknown rebuy field", fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":false,"extra":1}]}`, validRebuy), http.StatusBadRequest},
		{"malformed rebuy UUID", `{"rebuys":[{"id":"not-a-uuid","paid":true}]}`, http.StatusBadRequest},
		{"nil rebuy UUID", fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":true}]}`, uuid.Nil.String()), http.StatusBadRequest},
		{"numeric rebuy UUID", `{"rebuys":[{"id":1,"paid":true}]}`, http.StatusBadRequest},
		{"missing rebuy UUID", `{"rebuys":[{"paid":true}]}`, http.StatusBadRequest},
		{"null rebuy", `{"rebuys":[null]}`, http.StatusBadRequest},
		{"invalid rebuy payment", fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":"true"}]}`, validRebuy), http.StatusBadRequest},
		{"oversized array", `{"rebuys":[` + strings.Repeat(" ", maxRequestBytes) + `]}`, http.StatusRequestEntityTooLarge},
		{"oversized trailing whitespace", `{"buy_in_paid":true}` + strings.Repeat(" ", maxRequestBytes), http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newPlayerPatchHTTPFixture(t, domain.SettlementBank)
			before := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
			f.request(t, http.MethodPatch, f.path, tt.body, tt.status)
			assert.Equal(t, before, f.repo.cards[f.tournament.ID])
			assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
			assert.Len(t, f.repo.cards, 2)
		})
	}
}

func TestTournamentHTTPPlayerPatchTargetsSecondPlayer(t *testing.T) {
	f := newPlayerPatchHTTPFixture(t, domain.SettlementBank)
	f.path = "/tournaments/" + f.tournament.ID.String() + "/players/" + f.tournament.Players[1].ID.String()
	f.tournament.Players[1].BuyInPaid = false
	f.tournament.Players[1].Rebuys = []domain.Rebuy{}
	f.patch(t, `{"buy_in_paid":false,"rebuys":[]}`)
}

func TestTournamentHTTPPlayerPatchIDsAndMissingTarget(t *testing.T) {
	f := newPlayerPatchHTTPFixture(t, domain.SettlementBank)
	other, err := f.repo.GetByID(context.Background(), testUserID, f.otherID)
	require.NoError(t, err)
	tournamentPath := "/tournaments/" + f.tournament.ID.String() + "/players/"
	tests := []struct {
		name   string
		path   string
		status int
	}{
		{"malformed tournament UUID", "/tournaments/not-a-uuid/players/" + f.tournament.Players[0].ID.String(), http.StatusBadRequest},
		{"nil tournament UUID", "/tournaments/" + uuid.Nil.String() + "/players/" + f.tournament.Players[0].ID.String(), http.StatusBadRequest},
		{"malformed player UUID", tournamentPath + "not-a-uuid", http.StatusBadRequest},
		{"nil player UUID", tournamentPath + uuid.Nil.String(), http.StatusBadRequest},
		{"missing tournament", "/tournaments/" + uuid.New().String() + "/players/" + f.tournament.Players[0].ID.String(), http.StatusNotFound},
		{"missing player", tournamentPath + uuid.New().String(), http.StatusNotFound},
		{"player belongs to other tournament", tournamentPath + other.Players[0].ID.String(), http.StatusNotFound},
	}
	before := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.request(t, http.MethodPatch, tt.path, `{"buy_in_paid":false}`, tt.status)
			assert.Equal(t, before, f.repo.cards[f.tournament.ID])
			assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
			assert.Len(t, f.repo.cards, 2)
		})
	}
}

func TestTournamentHTTPPlayerPatchRejectsDuplicateRebuysWithoutMutation(t *testing.T) {
	for _, acrossPlayers := range []bool{false, true} {
		t.Run(fmt.Sprintf("across players %t", acrossPlayers), func(t *testing.T) {
			f := newPlayerPatchHTTPFixture(t, domain.SettlementBank)
			before := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
			id := uuid.New()
			rebuys := []RebuyRequest{{ID: id, Paid: false}, {ID: id, Paid: true}}
			if acrossPlayers {
				rebuys = []RebuyRequest{{ID: f.tournament.Players[1].Rebuys[0].ID, Paid: false}}
			}
			payload, err := json.Marshal(map[string]any{"buy_in_paid": false, "rebuys": rebuys})
			require.NoError(t, err)
			rec := f.request(t, http.MethodPatch, f.path, string(payload), http.StatusBadRequest)
			assert.Contains(t, rec.Body.String(), domain.ErrDuplicateRebuy.Error())
			assert.Equal(t, before, f.repo.cards[f.tournament.ID])
			assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
		})
	}
}

func TestTournamentHTTPPlayerPatchRejectsPotOverflowWithoutMutation(t *testing.T) {
	repo := &memoryTournamentRepository{cards: make(map[uuid.UUID][]byte)}
	router := newTournamentTestRouter(NewTournamentHandler(service.NewTournamentService(repo)))
	payload := strings.Replace(validCreateTournamentJSON(), `"buy_in_amount":1000`, fmt.Sprintf(`"buy_in_amount":%d`, int64(math.MaxInt64/2)), 1)
	create := httptest.NewRecorder()
	router.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(payload)))
	require.Equal(t, http.StatusCreated, create.Code, create.Body.String())
	var created CreateTournamentResponse
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))
	id := uuid.MustParse(created.ID)
	tr, err := repo.GetByID(context.Background(), testUserID, id)
	require.NoError(t, err)
	before := append([]byte(nil), repo.cards[id]...)
	path := "/tournaments/" + created.ID + "/players/" + tr.Players[0].ID.String()
	body := fmt.Sprintf(`{"buy_in_paid":true,"rebuys":[{"id":%q,"paid":false}]}`, uuid.New().String())
	patch := httptest.NewRecorder()
	router.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, patch.Code, patch.Body.String())
	assert.Contains(t, patch.Body.String(), domain.ErrPotOverflow.Error())
	assert.Equal(t, before, repo.cards[id])
	assert.Len(t, repo.cards, 1)
}

func TestTournamentHTTPPlayerPatchPreservesResultsAndRejectsPrizeMismatch(t *testing.T) {
	f := newPlayerPatchHTTPFixture(t, domain.SettlementP2P)
	settings := settingsRequestFixture()
	settings.Players = make([]PlayerRequest, 0, len(f.tournament.Players))
	settings.ChipSet = []ChipRequest{{Value: 25, Count: 80}, {Value: 100, Count: 80}}
	for _, player := range f.tournament.Players {
		rebuys := make([]RebuyRequest, 0, len(player.Rebuys))
		for _, rebuy := range player.Rebuys {
			rebuys = append(rebuys, RebuyRequest{ID: rebuy.ID, Paid: rebuy.Paid})
		}
		settings.Players = append(settings.Players, PlayerRequest{ID: player.ID, Name: player.Name, BuyInPaid: player.BuyInPaid, Rebuys: rebuys})
	}
	place := 1
	req := SaveTournamentRequest{TournamentSettingsRequest: settings, Results: []ResultRequest{
		{PlayerID: f.tournament.Players[0].ID, Place: &place, Prize: f.tournament.Pot()},
	}}
	before := append([]byte(nil), f.repo.cards[f.tournament.ID]...)
	req.Results[0].PlayerID = uuid.New()
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	invalid := f.request(t, http.MethodPut, "/tournaments/"+f.tournament.ID.String(), string(payload), http.StatusBadRequest)
	assert.Contains(t, invalid.Body.String(), domain.ErrPlayerNotFound.Error())
	assert.Equal(t, before, f.repo.cards[f.tournament.ID])
	req.Results[0].PlayerID = f.tournament.Players[0].ID
	payload, err = json.Marshal(req)
	require.NoError(t, err)
	f.request(t, http.MethodPut, "/tournaments/"+f.tournament.ID.String(), string(payload), http.StatusOK)
	f.tournament, err = f.repo.GetByID(context.Background(), testUserID, f.tournament.ID)
	require.NoError(t, err)
	f.tournament.Players[0].BuyInPaid = false
	paid := f.patch(t, `{"buy_in_paid":false}`)
	assert.True(t, paid.HasResults)
	f.tournament.Players[0].Rebuys[0].Paid = true
	body := fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":true}]}`, f.tournament.Players[0].Rebuys[0].ID.String())
	f.patch(t, body)
	before = append([]byte(nil), f.repo.cards[f.tournament.ID]...)
	for _, body := range []string{
		`{"buy_in_paid":true,"rebuys":[]}`,
		fmt.Sprintf(`{"buy_in_paid":true,"rebuys":[{"id":%q,"paid":true},{"id":%q,"paid":false}]}`, f.tournament.Players[0].Rebuys[0].ID.String(), uuid.New().String()),
	} {
		rec := f.request(t, http.MethodPatch, f.path, body, http.StatusBadRequest)
		assert.Contains(t, rec.Body.String(), domain.ErrPrizeSumMismatch.Error())
		assert.Equal(t, before, f.repo.cards[f.tournament.ID])
		assert.Equal(t, f.otherSnapshot, f.repo.cards[f.otherID])
	}
}

func TestTournamentPlayerPatchRequestToServiceInput(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		buyInPaid  *bool
		hasRebuys  bool
		rebuyCount int
		rebuyPaid  bool
	}{
		{name: "omitted fields", body: `{}`},
		{name: "null fields", body: `{"buy_in_paid":null,"rebuys":null}`},
		{name: "false payment", body: `{"buy_in_paid":false}`, buyInPaid: new(bool)},
		{name: "empty list", body: `{"rebuys":[]}`, hasRebuys: true},
		{name: "paid rebuy", body: fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":true}]}`, uuid.New().String()), hasRebuys: true, rebuyCount: 1, rebuyPaid: true},
		{name: "unpaid rebuy", body: fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":false}]}`, uuid.New().String()), hasRebuys: true, rebuyCount: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req PatchTournamentPlayerRequest
			require.NoError(t, json.Unmarshal([]byte(tt.body), &req))
			input, err := req.ToServiceInput()
			require.NoError(t, err)
			assert.Equal(t, tt.buyInPaid, input.BuyInPaid)
			if !tt.hasRebuys {
				assert.Nil(t, input.Rebuys)
				return
			}
			require.NotNil(t, input.Rebuys)
			require.NotNil(t, *input.Rebuys)
			require.Len(t, *input.Rebuys, tt.rebuyCount)
			if tt.rebuyCount > 0 {
				assert.Equal(t, (*req.Rebuys)[0].ID, (*input.Rebuys)[0].ID)
				assert.Equal(t, tt.rebuyPaid, (*input.Rebuys)[0].Paid)
				(*input.Rebuys)[0].Paid = !tt.rebuyPaid
				assert.Equal(t, tt.rebuyPaid, *(*req.Rebuys)[0].Paid)
			}
		})
	}
}

func TestTournamentPlayerPatchRequestToServiceInputRejectsMissingRebuyPayment(t *testing.T) {
	id := uuid.New().String()
	for _, tt := range []struct {
		name string
		body string
	}{
		{"null paid", fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":null}]}`, id)},
		{"missing paid", fmt.Sprintf(`{"rebuys":[{"id":%q}]}`, id)},
		{"invalid later rebuy", fmt.Sprintf(`{"buy_in_paid":false,"rebuys":[{"id":%q,"paid":false},{"id":%q}]}`, id, uuid.New().String())},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var req PatchTournamentPlayerRequest
			require.NoError(t, json.Unmarshal([]byte(tt.body), &req))
			input, err := req.ToServiceInput()
			require.Error(t, err)
			assert.Contains(t, err.Error(), ".paid is required and must be boolean")
			assert.Equal(t, service.PatchTournamentPlayerInput{}, input)
		})
	}
}

func TestTournamentHandler_PlayerPatch(t *testing.T) {
	tr := handlerTournamentFixture()
	path := "/tournaments/" + tr.ID.String() + "/players/" + tr.Players[0].ID.String()
	for _, body := range []string{
		`{"buy_in_paid":false}`, `{"buy_in_paid":true,"rebuys":null}`,
		`{"buy_in_paid":null,"rebuys":[]}`,
		fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":false}]}`, uuid.New().String()),
	} {
		t.Run(body, func(t *testing.T) {
			srv := &mockTournamentService{tournament: tr}
			rec := serve(t, srv, http.MethodPatch, path, body)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.True(t, srv.patchCalled)
			assert.Equal(t, tr.ID, srv.patchTournamentID)
			assert.Equal(t, tr.Players[0].ID, srv.patchPlayerID)
			var req PatchTournamentPlayerRequest
			require.NoError(t, json.Unmarshal([]byte(body), &req))
			input, err := req.ToServiceInput()
			require.NoError(t, err)
			assert.Equal(t, input, srv.patchInput)
			expected, err := newTournamentResponse(tr)
			require.NoError(t, err)
			var actual TournamentResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &actual))
			assert.Equal(t, expected, actual)
		})
	}
	t.Run("empty object reaches service validation", func(t *testing.T) {
		srv := &mockTournamentService{patchErr: errors.Join(app.ErrValidation, errors.New("empty patch"))}
		rec := serve(t, srv, http.MethodPatch, path, `{}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.True(t, srv.patchCalled)
		assert.Equal(t, service.PatchTournamentPlayerInput{}, srv.patchInput)
	})
	for _, tt := range []struct {
		name   string
		err    error
		status int
	}{
		{"missing tournament", app.ErrTournamentNotFound, http.StatusNotFound},
		{"missing player", domain.ErrPlayerNotFound, http.StatusNotFound},
		{"wrapped missing player", fmt.Errorf("patch player: %w", domain.ErrPlayerNotFound), http.StatusNotFound},
		{"validation takes precedence", errors.Join(app.ErrValidation, domain.ErrPlayerNotFound), http.StatusBadRequest},
		{"invalid tournament", app.ErrInvalidTournamentID, http.StatusBadRequest},
		{"storage failure", errors.New("db secret"), http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := &mockTournamentService{patchErr: tt.err}
			rec := serve(t, srv, http.MethodPatch, path, `{"buy_in_paid":true}`)
			assert.Equal(t, tt.status, rec.Code)
			assert.True(t, srv.patchCalled)
			assert.NotContains(t, rec.Body.String(), "db secret")
		})
	}
	for _, invalidPath := range []string{
		"/tournaments/not-a-uuid/players/" + tr.Players[0].ID.String(),
		"/tournaments/" + tr.ID.String() + "/players/not-a-uuid",
		"/tournaments/" + tr.ID.String() + "/players/" + uuid.Nil.String(),
	} {
		t.Run("invalid path "+invalidPath, func(t *testing.T) {
			srv := &mockTournamentService{}
			rec := serve(t, srv, http.MethodPatch, invalidPath, `{"buy_in_paid":true}`)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, srv.patchCalled)
		})
	}
	for _, body := range []string{
		"", `{`, `null`, `[]`, `{"buy_in_paid":"false"}`, `{"rebuys":{}}`,
		`{"buy_in_paid":true,"extra":1}`, `{"buy_in_paid":true}{}`,
		fmt.Sprintf(`{"rebuys":[{"id":%q,"paid":false,"extra":1}]}`, uuid.New().String()),
		`{"rebuys":[{"id":"not-a-uuid"}]}`,
	} {
		t.Run("strict decoder "+body, func(t *testing.T) {
			srv := &mockTournamentService{}
			rec := serve(t, srv, http.MethodPatch, path, body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, srv.patchCalled)
		})
	}
	t.Run("body limit", func(t *testing.T) {
		srv := &mockTournamentService{}
		rec := serve(t, srv, http.MethodPatch, path, `{"rebuys":[`+strings.Repeat(" ", maxRequestBytes)+`]}`)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		assert.False(t, srv.patchCalled)
	})
}
