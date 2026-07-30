package security

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTokenGenerator_Generate(t *testing.T) {
	t.Run("generates token and hash", func(t *testing.T) {
		generator := SessionTokenGenerator{}

		rawToken, tokenHash, err := generator.Generate()

		require.NoError(t, err)
		require.NotEmpty(t, rawToken)

		decodedToken, err := base64.RawURLEncoding.DecodeString(rawToken)
		require.NoError(t, err)
		assert.Len(t, decodedToken, 32)

		assert.Len(t, tokenHash, sha256.Size)

		expectedHash := sha256.Sum256([]byte(rawToken))
		assert.Equal(t, expectedHash[:], tokenHash)
	})
	t.Run("generates unique tokens", func(t *testing.T) {
		generator := SessionTokenGenerator{}

		firstToken, firstHash, err := generator.Generate()
		require.NoError(t, err)

		secondToken, secondHash, err := generator.Generate()
		require.NoError(t, err)

		assert.NotEqual(t, firstToken, secondToken)
		assert.NotEqual(t, firstHash, secondHash)
	})
}
