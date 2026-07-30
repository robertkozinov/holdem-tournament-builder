package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memory      = 19 * 1024
	iterations  = 2
	parallelism = 1
	saltLength  = 16
	keyLength   = 32
)

type PasswordHasher struct {
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, saltLength)

	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	return salt, nil
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	salt, err := generateSalt()
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	passwordHash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(passwordHash)

	result := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		iterations,
		parallelism,
		encodedSalt,
		encodedHash,
	)

	return result, nil
}

func (h *PasswordHasher) Verify(password string, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid password hash format")
	}

	if parts[0] != "" || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid password hash format")
	}

	var version int

	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse argon2 version: %w", err)
	}

	if version != argon2.Version {
		return false, fmt.Errorf("unsupported argon2 version: %d", version)
	}

	var (
		hashMemory      uint32
		hashIterations  uint32
		hashParallelism uint8
	)

	count, err := fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&hashMemory,
		&hashIterations,
		&hashParallelism,
	)
	if err != nil {
		return false, fmt.Errorf("parse argon2 parameters: %w", err)
	}
	if count != 3 {
		return false, fmt.Errorf("invalid argon2 parameters")
	}
	if hashMemory == 0 || hashIterations == 0 || hashParallelism == 0 {
		return false, fmt.Errorf("invalid argon2 parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	if len(salt) == 0 {
		return false, fmt.Errorf("salt is empty")
	}

	passwordHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}
	if len(passwordHash) == 0 {
		return false, fmt.Errorf("password hash is empty")
	}

	calculatedHash := argon2.IDKey(
		[]byte(password),
		salt,
		hashIterations,
		hashMemory,
		hashParallelism,
		uint32(len(passwordHash)),
	)

	return subtle.ConstantTimeCompare(passwordHash, calculatedHash) == 1, nil
}
