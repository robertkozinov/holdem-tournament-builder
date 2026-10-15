package http

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerRequest_InvalidUTF8(t *testing.T) {
	for _, body := range []string{
		"{\"name\":\"A\xffB\"}",
		`{"name":"Влад","user_id":"fake"}`,
	} {
		var req PlayerRequest
		require.Error(t, json.Unmarshal([]byte(body), &req))
	}
}

func TestPlayerRequest_ValidNameAndSelection(t *testing.T) {
	id := uuid.New()
	for _, body := range []string{
		`{"name":"Влад"}`,
		`{"name":"A�B"}`,
		`{"id":"` + id.String() + `"}`,
	} {
		var req PlayerRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		assert.True(t, req.ID == id || req.Name != "")
	}
}
