package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	transport "holdem-tournament-builder/internal/transport/http"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registryHTTPAuth struct{ owners map[uuid.UUID]bool }

func (s *registryHTTPAuth) Register(context.Context, string, string, time.Time) (domain.User, error) {
	return domain.User{}, app.ErrValidation
}
func (s *registryHTTPAuth) Login(context.Context, string, string, time.Time) (service.AuthResult, error) {
	return service.AuthResult{}, app.ErrValidation
}
func (s *registryHTTPAuth) Authenticate(_ context.Context, token string, _ time.Time) (uuid.UUID, error) {
	id, err := uuid.Parse(token)
	if err != nil || !s.owners[id] {
		return uuid.Nil, app.ErrUnauthenticated
	}
	return id, nil
}
func (s *registryHTTPAuth) Logout(context.Context, string) error { return nil }
func (s *registryHTTPAuth) CurrentUser(_ context.Context, id uuid.UUID) (domain.User, error) {
	return domain.User{ID: id}, nil
}

func TestPlayerRegistryHTTP_Integration(t *testing.T) {
	ctx, pool, profiles, owner := setupPlayerRepositoryTest(t)
	_, _, _, foreign := setupPlayerRepositoryTest(t)
	router := transport.NewRouter(
		transport.NewTournamentHandler(service.NewTournamentService(NewTournamentRepository(pool))),
		transport.NewAuthHandler(&registryHTTPAuth{owners: map[uuid.UUID]bool{owner: true, foreign: true}}, transport.CookieOptions{}, config.DefaultRequestSafety()),
		transport.NewPlayerHandler(service.NewPlayerService(profiles)),
	)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	request := func(account uuid.UUID, method, path string, body any, status int) []byte {
		t.Helper()
		var data []byte
		var err error
		if body != nil {
			data, err = json.Marshal(body)
			require.NoError(t, err)
		}
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(data))
		require.NoError(t, err)
		if account != uuid.Nil {
			req.AddCookie(&http.Cookie{Name: "holdem_session", Value: account.String()})
		}
		res, err := server.Client().Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		data, err = io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, status, res.StatusCode, string(data))
		return data
	}
	createPlayer := func(account uuid.UUID, name string) transport.RegistryPlayerResponse {
		t.Helper()
		var p transport.RegistryPlayerResponse
		require.NoError(t, json.Unmarshal(request(account, "POST", "/players", map[string]string{"name": name}, 201), &p))
		return p
	}
	request(uuid.Nil, "GET", "/players", nil, 401)
	vlad := createPlayer(owner, " Влад ")
	request(owner, "POST", "/players", map[string]string{"name": "ВЛАД"}, 409)
	foreignVlad := createPlayer(foreign, "влад")
	assert.NotEqual(t, vlad.ID, foreignVlad.ID)
	create := transport.CreateTournamentRequest{
		TournamentSettingsRequest: transport.TournamentSettingsRequest{
			Name: "Registry integration", Date: time.Date(2026, 10, 13, 20, 0, 0, 0, time.UTC),
			Players: []transport.PlayerRequest{{ID: vlad.ID, Name: "untrusted client name"}, {Name: "Артём"}}, SettlementMode: "p2p",
			ChipSet: []transport.ChipRequest{{Value: 25, Count: 100}, {Value: 100, Count: 100}}, Style: "standard", DurationMinutes: 120, LevelDurationMinutes: 20,
		}, BuyInAmount: 1000,
	}
	var created transport.CreateTournamentResponse
	require.NoError(t, json.Unmarshal(request(owner, "POST", "/tournaments", create, 201), &created))
	path := "/tournaments/" + created.ID
	readCard := func() transport.TournamentResponse {
		t.Helper()
		var card transport.TournamentResponse
		require.NoError(t, json.Unmarshal(request(owner, "GET", path, nil, 200), &card))
		return card
	}
	card := readCard()
	require.Len(t, card.Players, 2)
	assert.Equal(t, vlad.ID.String(), card.Players[0].ID)
	assert.Equal(t, "Влад", card.Players[0].Name)
	request(owner, "PATCH", path+"/players/"+vlad.ID.String(), map[string]any{"rebuys": []map[string]any{{"id": uuid.NewString(), "paid": true}}}, 200)
	card = readCard()
	assert.EqualValues(t, 3000, card.Pot)
	firstPlace := 1
	save := transport.SaveTournamentRequest{TournamentSettingsRequest: create.TournamentSettingsRequest}
	save.Players = []transport.PlayerRequest{
		{ID: vlad.ID, Name: "not a rename", Rebuys: []transport.RebuyRequest{{ID: uuid.MustParse(card.Players[0].Rebuys[0].ID), Paid: true}}},
		{ID: uuid.MustParse(card.Players[1].ID)},
	}
	save.Results = []transport.ResultRequest{{PlayerID: vlad.ID, Place: &firstPlace, Prize: 3000}}
	request(owner, "PUT", path, save, 200)
	var before, after []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT data FROM tournaments WHERE id = $1", created.ID).Scan(&before))
	request(owner, "PATCH", "/players/"+vlad.ID.String(), map[string]string{"name": "Влад Фед"}, 200)
	require.NoError(t, pool.QueryRow(ctx, "SELECT data FROM tournaments WHERE id = $1", created.ID).Scan(&after))
	assert.JSONEq(t, string(before), string(after))
	card = readCard()
	assert.Equal(t, "Влад Фед", card.Players[0].Name)
	require.Len(t, card.Results, 1)
	assert.Equal(t, "Влад Фед", card.Results[0].Name)
	var listed []transport.TournamentResponse
	require.NoError(t, json.Unmarshal(request(owner, "GET", "/tournaments", nil, 200), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, "Влад Фед", listed[0].Players[0].Name)
	request(foreign, "PATCH", "/players/"+vlad.ID.String(), map[string]string{"name": "Чужой"}, 404)
	request(foreign, "DELETE", "/players/"+vlad.ID.String(), nil, 404)
	request(owner, "DELETE", "/players/"+vlad.ID.String(), nil, 204)
	card = readCard()
	assert.True(t, card.Players[0].IsDeleted)
	assert.Equal(t, "Влад Фед", card.Players[0].Name)
	assert.True(t, card.Results[0].IsDeleted)
	assert.EqualValues(t, 3000, card.Pot)
	assert.Len(t, card.Players[0].Rebuys, 1)
	request(owner, "PATCH", "/players/"+vlad.ID.String(), map[string]string{"name": "Restored"}, 404)
	recreated := createPlayer(owner, "Влад Фед")
	assert.NotEqual(t, vlad.ID, recreated.ID)
	request(owner, "POST", "/tournaments", create, 404)
	create.Players = []transport.PlayerRequest{{ID: foreignVlad.ID}, {Name: "Must rollback"}}
	request(owner, "POST", "/tournaments", create, 404)
	active, err := profiles.List(ctx, owner, "must rollback", 50, 0)
	require.NoError(t, err)
	assert.Empty(t, active)
	save.Players = append(save.Players, transport.PlayerRequest{ID: recreated.ID})
	save.Results[0].Prize = 4000
	request(owner, "PUT", path, save, 200)
	card = readCard()
	require.Len(t, card.Players, 3)
	assert.Equal(t, card.Players[0].Name, card.Players[2].Name)
	assert.True(t, card.Players[0].IsDeleted)
	assert.False(t, card.Players[2].IsDeleted)
	require.Len(t, card.Transfers, 2)
	for _, transfer := range card.Transfers {
		assert.Equal(t, vlad.ID.String(), transfer.ToPlayerID)
		assert.EqualValues(t, 1000, transfer.Amount)
	}
	assert.EqualValues(t, 4000, card.Pot)
}
