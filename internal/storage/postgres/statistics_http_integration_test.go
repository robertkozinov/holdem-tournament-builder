package postgres

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	transport "holdem-tournament-builder/internal/transport/http"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticsHTTPAccountIsolationIntegration(t *testing.T) {
	ctx, pool, _, owner := setupPlayerRepositoryTest(t)
	_, _, _, foreign := setupPlayerRepositoryTest(t)
	repo := NewTournamentRepository(pool)

	completeTournament := func(account uuid.UUID, winnerName, loserName string) []uuid.UUID {
		t.Helper()
		card, err := repo.SaveWithPlayers(ctx, account, uuid.Nil, []domain.TournamentPlayer{
			{Name: winnerName},
			{Name: loserName},
		}, registryTournamentBuilder(validTournament(t)))
		require.NoError(t, err)
		firstPlace := 1
		card.Results = []domain.Result{{PlayerID: card.Players[0].ID, Place: &firstPlace, Prize: 2000}}
		require.NoError(t, repo.Update(ctx, account, card))
		return []uuid.UUID{card.Players[0].ID, card.Players[1].ID}
	}

	ownerPlayers := completeTournament(owner, "Owner Winner", "Owner Loser")
	foreignPlayers := completeTournament(foreign, "Foreign Winner", "Foreign Loser")
	router := transport.NewRouter(
		transport.NewTournamentHandler(service.NewTournamentService(repo)),
		transport.NewAuthHandler(&registryHTTPAuth{owners: map[uuid.UUID]bool{owner: true, foreign: true}}, transport.CookieOptions{}, config.DefaultRequestSafety()),
		transport.NewPlayerHandler(service.NewPlayerService(NewPlayerRepository(pool))),
		transport.NewStatisticsHandler(service.NewStatisticsService(NewStatisticsRepository(pool))),
	)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	leaderboard := func(account uuid.UUID, status int) []transport.PlayerStatisticsResponse {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/statistics/leaderboard", nil)
		require.NoError(t, err)
		if account != uuid.Nil {
			req.AddCookie(&http.Cookie{Name: "holdem_session", Value: account.String()})
		}
		res, err := server.Client().Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, status, res.StatusCode, string(data))
		if status != http.StatusOK {
			return nil
		}
		var items []transport.PlayerStatisticsResponse
		require.NoError(t, json.Unmarshal(data, &items))
		return items
	}

	leaderboard(uuid.Nil, http.StatusUnauthorized)
	ownerItems := leaderboard(owner, http.StatusOK)
	foreignItems := leaderboard(foreign, http.StatusOK)
	require.Len(t, ownerItems, 2)
	require.Len(t, foreignItems, 2)
	assert.ElementsMatch(t, ownerPlayers, statisticsPlayerIDs(ownerItems))
	assert.ElementsMatch(t, foreignPlayers, statisticsPlayerIDs(foreignItems))
	assert.ElementsMatch(t, []string{"Owner Winner", "Owner Loser"}, statisticsPlayerNames(ownerItems))
	assert.ElementsMatch(t, []string{"Foreign Winner", "Foreign Loser"}, statisticsPlayerNames(foreignItems))
	for _, item := range ownerItems {
		assert.NotContains(t, foreignPlayers, item.PlayerID)
	}
}

func TestStatisticsHTTPOverflowIntegration(t *testing.T) {
	ctx, pool, _, owner := setupPlayerRepositoryTest(t)
	repo := NewTournamentRepository(pool)
	template := validTournament(t)
	template.BuyInAmount = 3000000000000000000
	router := transport.NewRouter(
		transport.NewTournamentHandler(service.NewTournamentService(repo)),
		transport.NewAuthHandler(&registryHTTPAuth{owners: map[uuid.UUID]bool{owner: true}}, transport.CookieOptions{}, config.DefaultRequestSafety()),
		transport.NewPlayerHandler(service.NewPlayerService(NewPlayerRepository(pool))),
		transport.NewStatisticsHandler(service.NewStatisticsService(NewStatisticsRepository(pool))),
	)
	for _, totals := range []struct {
		prizes, contribution string
	}{
		{prizes: "12000000000000000000", contribution: "6000000000000000000"},
		{prizes: "24000000000000000000", contribution: "12000000000000000000"},
	} {
		for range 2 {
			card, err := repo.SaveWithPlayers(ctx, owner, uuid.Nil, []domain.TournamentPlayer{
				{Name: "Winner"}, {Name: "Loser"},
			}, registryTournamentBuilder(template))
			require.NoError(t, err)
			firstPlace := 1
			card.Results = []domain.Result{{PlayerID: card.Players[0].ID, Place: &firstPlace, Prize: 6000000000000000000}}
			_, err = domain.NewTournament(*card)
			require.NoError(t, err)
			require.NoError(t, repo.Update(ctx, owner, card))
		}
		r := httptest.NewRequest(http.MethodGet, "/statistics/leaderboard", nil)
		r.AddCookie(&http.Cookie{Name: "holdem_session", Value: owner.String()})
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		decoder := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
		decoder.UseNumber()
		var items []map[string]any
		require.NoError(t, decoder.Decode(&items))
		require.Len(t, items, 2)
		assert.Equal(t, "Winner", items[0]["name"])
		assert.Equal(t, "Loser", items[1]["name"])
		for i, want := range []map[string]string{
			{"total_buy_ins": totals.contribution, "total_prizes": totals.prizes, "net_profit": totals.contribution},
			{"total_buy_ins": totals.contribution, "total_prizes": "0", "net_profit": "-" + totals.contribution},
		} {
			for field, decimal := range want {
				number, ok := items[i][field].(json.Number)
				require.True(t, ok, "%s must be a JSON number, got %T", field, items[i][field])
				assert.Equal(t, decimal, number.String(), field)
			}
		}
	}
}

func statisticsPlayerIDs(items []transport.PlayerStatisticsResponse) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.PlayerID)
	}
	return ids
}

func statisticsPlayerNames(items []transport.PlayerStatisticsResponse) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}
