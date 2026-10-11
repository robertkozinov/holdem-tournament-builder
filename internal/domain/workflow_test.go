package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentHasNoRuntimeState(t *testing.T) {
	data, err := json.Marshal(Tournament{})
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fields))
	assert.NotContains(t, fields, "Status")
	assert.NotContains(t, fields, "PayoutSpots")
	assert.NotContains(t, fields, "Contributions")
}
