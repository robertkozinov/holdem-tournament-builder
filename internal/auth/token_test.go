package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionTokenGeneratorRoundTrip(t *testing.T) {
	generator := SessionTokenGenerator{}
	seen := make(map[string]bool)
	for range 32 {
		raw, hash, err := generator.Generate()
		require.NoError(t, err)
		require.Len(t, raw, 43)
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
		require.NoError(t, err)
		require.Len(t, decoded, 32)
		require.Equal(t, raw, base64.RawURLEncoding.EncodeToString(decoded))
		require.False(t, seen[raw])
		seen[raw] = true
		sum := sha256.Sum256([]byte(raw))
		require.Equal(t, hex.EncodeToString(sum[:]), hash)
		require.NotEqual(t, raw, hash)
		actual, err := generator.Hash(raw)
		require.NoError(t, err)
		require.Equal(t, hash, actual)
	}
}

func TestSessionTokenGeneratorRejectsMalformedTokens(t *testing.T) {
	canonical := strings.Repeat("A", 43)
	for _, raw := range []string{
		"", "bad", strings.Repeat("A", 42), strings.Repeat("A", 44),
		canonical + "=", canonical[:42] + "B", "+" + canonical[1:],
		"/" + canonical[1:], canonical[:20] + "\n" + canonical[20:],
		canonical[:20] + " " + canonical[21:], strings.Repeat("A", 100000),
	} {
		hash, err := (SessionTokenGenerator{}).Hash(raw)
		require.Error(t, err)
		require.Empty(t, hash)
	}
}

func TestSessionTokenGeneratorRandomFailure(t *testing.T) {
	cause := errors.New("random source unavailable")
	raw, hash, err := (SessionTokenGenerator{random: failingRandomReader{cause}}).Generate()
	require.ErrorIs(t, err, cause)
	require.Empty(t, raw)
	require.Empty(t, hash)
}
