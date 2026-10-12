package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
)

func TestPasswordHasherRoundTrip(t *testing.T) {
	hasher := PasswordHasher{}
	password := "Case-Sensitive!9"
	first, err := hasher.Hash(password)
	require.NoError(t, err)
	second, err := hasher.Hash(password)
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	parts := strings.Split(first, "$")
	require.Len(t, parts, 6)
	require.Equal(t, []string{"", "argon2id", "v=19", "m=19456,t=2,p=1"}, parts[:4])
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	require.NoError(t, err)
	require.Len(t, salt, 16)
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	require.NoError(t, err)
	require.Len(t, key, 32)
	require.Equal(t, argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32), key)
	for _, input := range []struct {
		password string
		want     bool
	}{{password, true}, {"case-sensitive!9", false}, {"wrong-password", false}, {" " + password, false}} {
		ok, err := hasher.Verify(input.password, first)
		require.NoError(t, err)
		require.Equal(t, input.want, ok)
	}
}

func TestPasswordHasherRejectsMalformedPHC(t *testing.T) {
	salt := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef"))
	key := base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	valid := "$argon2id$v=19$m=19456,t=2,p=1$" + salt + "$" + key
	cases := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"no metadata", "invalid"},
		{"algorithm", strings.Replace(valid, "argon2id", "argon2i", 1)},
		{"version", strings.Replace(valid, "v=19", "v=16", 1)},
		{"memory high", strings.Replace(valid, "m=19456", "m=4294967295", 1)},
		{"memory low", strings.Replace(valid, "m=19456", "m=1", 1)},
		{"different memory", strings.Replace(valid, "m=19456", "m=19455", 1)},
		{"iterations", strings.Replace(valid, "t=2", "t=4294967295", 1)},
		{"different iterations", strings.Replace(valid, "t=2", "t=3", 1)},
		{"parallelism", strings.Replace(valid, "p=1", "p=255", 1)},
		{"different parallelism", strings.Replace(valid, "p=1", "p=2", 1)},
		{"zero parallelism", strings.Replace(valid, "p=1", "p=0", 1)},
		{"reordered", strings.Replace(valid, "m=19456,t=2,p=1", "t=2,m=19456,p=1", 1)},
		{"extra parameter", strings.Replace(valid, "p=1", "p=1,x=1", 1)},
		{"leading zero", strings.Replace(valid, "t=2", "t=02", 1)},
		{"missing key", strings.TrimSuffix(valid, "$"+key)},
		{"extra field", valid + "$extra"},
		{"huge input", strings.Repeat("x", 100000)},
		{"short salt", strings.Replace(valid, salt, salt[:21], 1)},
		{"short key", strings.Replace(valid, key, key[:42], 1)},
		{"long salt", strings.Replace(valid, salt, salt+"A", 1)},
		{"long key", valid + "A"},
		{"salt padding", strings.Replace(valid, salt, salt+"==", 1)},
		{"key padding", valid + "="},
		{"bad salt alphabet", strings.Replace(valid, salt, "!"+salt[1:], 1)},
		{"bad key alphabet", strings.Replace(valid, key, "!"+key[1:], 1)},
		{"salt trailing bits", strings.Replace(valid, salt, salt[:21]+"h", 1)},
		{"key trailing bits", strings.Replace(valid, key, key[:42]+"B", 1)},
		{"newline", strings.Replace(valid, salt, salt[:10]+"\n"+salt[10:], 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := (PasswordHasher{}).Verify("test-password", tc.hash)
			require.Error(t, err)
			require.False(t, ok)
		})
	}
}

type failingRandomReader struct{ err error }

func (r failingRandomReader) Read([]byte) (int, error) { return 0, r.err }

func TestPasswordHasherRandomFailure(t *testing.T) {
	cause := errors.New("random source unavailable")
	hash, err := (PasswordHasher{random: failingRandomReader{cause}}).Hash("test-password")
	require.ErrorIs(t, err, cause)
	require.Empty(t, hash)
	require.NotContains(t, err.Error(), "test-password")
}
