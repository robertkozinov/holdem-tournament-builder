package http

import (
	"encoding/json"
	"testing"

	"holdem-tournament-builder/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentResponseRegistryIdentity(t *testing.T) {
	tr := handlerTournamentFixture()
	tr.Players[0].IsDeleted = true
	tr.Results = []domain.Result{{PlayerID: tr.Players[0].ID, Prize: tr.Pot()}}
	response, err := newTournamentResponse(tr)
	require.NoError(t, err)
	data, err := json.Marshal(response)
	require.NoError(t, err)
	var fields struct {
		Players   []map[string]any `json:"players"`
		Results   []map[string]any `json:"results"`
		Transfers []map[string]any `json:"transfers"`
	}
	require.NoError(t, json.Unmarshal(data, &fields))
	assert.Equal(t, true, fields.Players[0]["is_deleted"])
	assert.Equal(t, true, fields.Results[0]["is_deleted"])
	require.NotEmpty(t, fields.Transfers)
	assert.Equal(t, tr.Players[0].ID.String(), fields.Transfers[0]["to_player_id"])
	assert.Equal(t, tr.Players[1].ID.String(), fields.Transfers[0]["from_player_id"])
	assert.Equal(t, true, fields.Transfers[0]["to_is_deleted"])
	assert.Equal(t, false, fields.Transfers[0]["from_is_deleted"])
}
