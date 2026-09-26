package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenLen = 32

type SessionTokenGenerator struct {
}

func (g *SessionTokenGenerator) Generate() (rawToken string, tokenHash []byte, err error) {
	token := make([]byte, tokenLen)

	if _, err := rand.Read(token); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}

	rawToken = base64.RawURLEncoding.EncodeToString(token)
	tokenHash = g.Hash(rawToken)

	return rawToken, tokenHash, nil
}

func (g *SessionTokenGenerator) Hash(rawToken string) []byte {
	hash := sha256.Sum256([]byte(rawToken))
	return hash[:]
}
