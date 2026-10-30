package http

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statisticsServiceStub struct {
	called        bool
	calls         int
	limit, offset int
	items         []domain.PlayerStatistics
	err           error
}

func (s *statisticsServiceStub) Leaderboard(_ context.Context, limit, offset int) ([]domain.PlayerStatistics, error) {
	s.called, s.limit, s.offset = true, limit, offset
	s.calls++
	return s.items, s.err
}

func newStatisticsTestHandler() *StatisticsHandler {
	return NewStatisticsHandler(&statisticsServiceStub{})
}

func statisticsHandlerRequest(handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestStatisticsHandler_LeaderboardPagination(t *testing.T) {
	for _, tc := range []struct {
		name          string
		target        string
		limit, offset int
	}{
		{name: "defaults", target: "/statistics/leaderboard", limit: 50, offset: 0},
		{name: "explicit", target: "/statistics/leaderboard?limit=100&offset=25", limit: 100, offset: 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &statisticsServiceStub{}
			w := statisticsHandlerRequest(NewStatisticsHandler(s).Leaderboard, tc.target)

			require.Equal(t, http.StatusOK, w.Code)
			assert.True(t, s.called)
			assert.Equal(t, tc.limit, s.limit)
			assert.Equal(t, tc.offset, s.offset)
			assert.JSONEq(t, "[]", w.Body.String())
		})
	}
}

func TestStatisticsHandler_LeaderboardRejectsInvalidPagination(t *testing.T) {
	for _, query := range []string{
		"limit=", "limit=no", "limit=0", "limit=101", "limit=1&limit=2",
		"offset=", "offset=no", "offset=-1", "offset=0&offset=1",
	} {
		t.Run(query, func(t *testing.T) {
			s := &statisticsServiceStub{}
			w := statisticsHandlerRequest(NewStatisticsHandler(s).Leaderboard, "/statistics/leaderboard?"+query)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.False(t, s.called)
		})
	}
}

func TestStatisticsHandler_LeaderboardResponse(t *testing.T) {
	playerID := uuid.MustParse("00000000-0000-0000-0000-000000000321")
	s := &statisticsServiceStub{items: []domain.PlayerStatistics{{
		PlayerID:          playerID,
		Name:              "Влад",
		TournamentsPlayed: 3,
		TotalBuyIns:       big.NewInt(5000),
		TotalPrizes:       big.NewInt(3500),
		NetProfit:         big.NewInt(-1500),
	}}}

	w := statisticsHandlerRequest(NewStatisticsHandler(s).Leaderboard, "/statistics/leaderboard")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var items []PlayerStatisticsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Equal(t, []PlayerStatisticsResponse{{
		PlayerID:          playerID,
		Name:              "Влад",
		TournamentsPlayed: 3,
		TotalBuyIns:       big.NewInt(5000),
		TotalPrizes:       big.NewInt(3500),
		NetProfit:         big.NewInt(-1500),
	}}, items)
	var fields []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fields))
	require.Len(t, fields, 1)
	assert.ElementsMatch(t, []string{
		"player_id", "name", "tournaments_played", "total_buy_ins", "total_prizes", "net_profit",
	}, mapKeys(fields[0]))
}

func TestStatisticsHandler_LeaderboardLargeNumbers(t *testing.T) {
	buyIns, ok := new(big.Int).SetString("24000000000000000000", 10)
	require.True(t, ok)
	prizes, ok := new(big.Int).SetString("12000000000000000000", 10)
	require.True(t, ok)
	profit, ok := new(big.Int).SetString("-12000000000000000000", 10)
	require.True(t, ok)
	s := &statisticsServiceStub{items: []domain.PlayerStatistics{{
		PlayerID: uuid.New(), TotalBuyIns: buyIns, TotalPrizes: prizes, NetProfit: profit,
	}}}

	w := statisticsHandlerRequest(NewStatisticsHandler(s).Leaderboard, "/statistics/leaderboard")

	require.Equal(t, http.StatusOK, w.Code)
	decoder := json.NewDecoder(w.Body)
	decoder.UseNumber()
	var items []map[string]any
	require.NoError(t, decoder.Decode(&items))
	require.Len(t, items, 1)
	for field, want := range map[string]string{
		"total_buy_ins": "24000000000000000000",
		"total_prizes":  "12000000000000000000",
		"net_profit":    "-12000000000000000000",
	} {
		number, ok := items[0][field].(json.Number)
		require.True(t, ok, "%s must be a JSON number, got %T", field, items[0][field])
		assert.Equal(t, want, number.String(), field)
	}
}

func TestStatisticsHandler_LeaderboardErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "validation", err: app.ErrValidation, status: http.StatusBadRequest},
		{name: "internal", err: errors.New("private database detail"), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := statisticsHandlerRequest(NewStatisticsHandler(&statisticsServiceStub{err: tc.err}).Leaderboard, "/statistics/leaderboard")

			assert.Equal(t, tc.status, w.Code)
			assert.NotContains(t, w.Body.String(), "private database detail")
		})
	}
}

func TestLeaderboardRoute_Protected(t *testing.T) {
	for _, tc := range []struct {
		name, token string
		authErr     error
		status      int
	}{
		{name: "no cookie", status: http.StatusUnauthorized},
		{name: "expired session", token: "expired", authErr: app.ErrUnauthenticated, status: http.StatusUnauthorized},
		{name: "session unavailable", token: "unavailable", authErr: app.ErrSessionUnavailable, status: http.StatusServiceUnavailable},
		{name: "authenticated", token: "valid", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &statisticsServiceStub{}
			accounts := &fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) {
				return testUserID, tc.authErr
			}}
			router := NewRouter(
				NewTournamentHandler(&mockTournamentService{}),
				NewAuthHandler(accounts, CookieOptions{}, config.DefaultRequestSafety()),
				NewPlayerHandler(&playerServiceStub{}),
				NewStatisticsHandler(s),
			)
			r := httptest.NewRequest(http.MethodGet, "/statistics/leaderboard", nil)
			if tc.token != "" {
				r.AddCookie(&http.Cookie{Name: "holdem_session", Value: tc.token})
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			assert.Equal(t, tc.status, w.Code)
			assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			assert.Equal(t, tc.status == http.StatusOK, s.called)
		})
	}
}

func TestLeaderboardRoute_RateLimitBeforeSessionLookup(t *testing.T) {
	authCalls := 0
	accounts := &fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) {
		authCalls++
		return testUserID, nil
	}}
	safety := config.DefaultRequestSafety()
	safety.General.Burst = 1
	auth := NewAuthHandler(accounts, CookieOptions{}, safety)
	auth.now = func() time.Time { return time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC) }
	statistics := &statisticsServiceStub{}
	router := NewRouter(
		NewTournamentHandler(&mockTournamentService{}),
		auth,
		NewPlayerHandler(&playerServiceStub{}),
		NewStatisticsHandler(statistics),
	)

	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests} {
		r := httptest.NewRequest(http.MethodGet, "/statistics/leaderboard", nil)
		r.AddCookie(&http.Cookie{Name: "holdem_session", Value: "valid"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		assert.Equal(t, status, w.Code)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
	assert.Equal(t, 1, authCalls)
	assert.Equal(t, 1, statistics.calls)
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
