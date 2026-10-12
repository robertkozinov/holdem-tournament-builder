package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

type PasswordHasher struct{ random io.Reader }

const passwordHashPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"

var errInvalidPasswordHash = errors.New("invalid password hash")

func (h PasswordHasher) Hash(password string) (string, error) {
	reader := h.random
	if reader == nil {
		reader = rand.Reader
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(reader, salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	return passwordHashPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func (PasswordHasher) Verify(password, encodedHash string) (bool, error) {
	if len(encodedHash) != len(passwordHashPrefix)+22+1+43 || !strings.HasPrefix(encodedHash, passwordHashPrefix) {
		return false, errInvalidPasswordHash
	}
	saltText, keyText, ok := strings.Cut(encodedHash[len(passwordHashPrefix):], "$")
	if !ok || len(saltText) != 22 || len(keyText) != 43 {
		return false, errInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(saltText)
	if err != nil {
		return false, fmt.Errorf("%w: decode salt: %w", errInvalidPasswordHash, err)
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(keyText)
	if err != nil {
		return false, fmt.Errorf("%w: decode key: %w", errInvalidPasswordHash, err)
	}
	if len(salt) != 16 || len(key) != 32 || base64.RawStdEncoding.EncodeToString(salt) != saltText || base64.RawStdEncoding.EncodeToString(key) != keyText {
		return false, errInvalidPasswordHash
	}
	actual := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	return subtle.ConstantTimeCompare(actual, key) == 1, nil
}
