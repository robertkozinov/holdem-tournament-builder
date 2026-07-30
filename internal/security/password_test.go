package security

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHasher_Hash(t *testing.T) {
	t.Run("hashes password", func(t *testing.T) {
		password := "RiverCard7!"

		hasher := PasswordHasher{}
		hash, err := hasher.Hash(password)

		require.NoError(t, err)
		assert.NotEqual(t, password, hash)

		parts := strings.Split(hash, "$")

		require.Len(t, parts, 6)
		assert.Empty(t, parts[0])
		assert.Equal(t, "argon2id", parts[1])
		assert.Equal(t, "v=19", parts[2])
		assert.Equal(t, "m=19456,t=2,p=1", parts[3])

		salt, err := base64.RawStdEncoding.DecodeString(parts[4])
		require.NoError(t, err)
		assert.Len(t, salt, saltLength)

		passwordHash, err := base64.RawStdEncoding.DecodeString(parts[5])
		require.NoError(t, err)
		assert.Len(t, passwordHash, keyLength)
	})
	t.Run("uses unique salt", func(t *testing.T) {
		hasher := PasswordHasher{}

		firstHash, err := hasher.Hash("RiverCard7!")
		require.NoError(t, err)

		secondHash, err := hasher.Hash("RiverCard7!")
		require.NoError(t, err)

		assert.NotEqual(t, firstHash, secondHash)
	})
}

func TestPasswordHasher_Verify(t *testing.T) {
	t.Run("returns true for correct password", func(t *testing.T) {
		password := "RiverCard7!"
		hasher := PasswordHasher{}

		hash, err := hasher.Hash(password)
		require.NoError(t, err)

		res, err := hasher.Verify(password, hash)

		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("returns false for incorrect password", func(t *testing.T) {
		password := "RiverCard7!"
		hasher := PasswordHasher{}

		hash, err := hasher.Hash(password)
		require.NoError(t, err)

		res, err := hasher.Verify("WrongPassword7!", hash)

		require.NoError(t, err)
		assert.False(t, res)
	})

	tests := []struct {
		name        string
		encodedHash string
	}{
		{
			name:        "invalid format",
			encodedHash: "not-a-password-hash",
		},
		{
			name:        "unknown algorithm",
			encodedHash: "$bcrypt$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		},
		{
			name:        "unsupported version",
			encodedHash: "$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		},
		{
			name:        "invalid parameters",
			encodedHash: "$argon2id$v=19$m=invalid,t=2,p=1$c2FsdA$aGFzaA",
		},
		{
			name:        "zero parameters",
			encodedHash: "$argon2id$v=19$m=0,t=0,p=0$c2FsdA$aGFzaA",
		},
		{
			name:        "invalid salt encoding",
			encodedHash: "$argon2id$v=19$m=19456,t=2,p=1$%%%$aGFzaA",
		},
		{
			name:        "invalid password hash encoding",
			encodedHash: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$%%%",
		},
		{
			name:        "empty salt",
			encodedHash: "$argon2id$v=19$m=19456,t=2,p=1$$aGFzaA",
		},
		{
			name:        "empty password hash",
			encodedHash: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := PasswordHasher{}

			res, err := hasher.Verify("RiverCard7!", tt.encodedHash)

			require.Error(t, err)
			assert.False(t, res)
		})
	}
}
