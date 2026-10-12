package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

type SessionTokenGenerator struct{ random io.Reader }

var ErrInvalidToken = errors.New("invalid session token")

func (g SessionTokenGenerator) Generate() (string, string, error) {
	reader := g.random
	if reader == nil {
		reader = rand.Reader
	}
	data := make([]byte, 32)
	if _, err := io.ReadFull(reader, data); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(data)
	hash, err := g.Hash(raw)
	if err != nil {
		return "", "", err
	}
	return raw, hash, nil
}

func (SessionTokenGenerator) Hash(raw string) (string, error) {
	if len(raw) != 43 {
		return "", ErrInvalidToken
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if len(data) != 32 || base64.RawURLEncoding.EncodeToString(data) != raw {
		return "", ErrInvalidToken
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}
