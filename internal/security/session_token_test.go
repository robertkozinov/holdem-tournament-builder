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

func TestSessionTokenGenerator_Hash(t *testing.T) {
	t.Run("hashes token", func(t *testing.T) {
		generator := SessionTokenGenerator{}

		rawToken := "session-token"

		tokenHash := generator.Hash(rawToken)
		assert.NotNil(t, tokenHash)
		assert.Len(t, tokenHash, sha256.Size)

		expectedHash := sha256.Sum256([]byte(rawToken))
		assert.Equal(t, expectedHash[:], tokenHash)
	})
	t.Run("different raw tokens give different hashes", func(t *testing.T) {
		generator := SessionTokenGenerator{}

		firstRawToken := "first-session-token"
		secondRawToken := "second-session-token"

		firstTokenHash := generator.Hash(firstRawToken)
		secondTokenHash := generator.Hash(secondRawToken)

		assert.NotNil(t, firstTokenHash)
		assert.NotNil(t, secondTokenHash)

		assert.NotEqual(t, firstTokenHash, secondTokenHash)
	})
	t.Run("same raw tokens give same hashes", func(t *testing.T) {
		generator := SessionTokenGenerator{}

		firstRawToken := "session-token"
		secondRawToken := "session-token"

		firstTokenHash := generator.Hash(firstRawToken)
		secondTokenHash := generator.Hash(secondRawToken)

		assert.NotNil(t, firstTokenHash)
		assert.NotNil(t, secondTokenHash)

		assert.Equal(t, firstTokenHash, secondTokenHash)
	})
}
