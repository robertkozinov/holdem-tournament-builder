package service

import (
	"fmt"
	"strings"

	"holdem-tournament-builder/internal/app"
)

func normalizeAuthLogin(login string) (string, error) {
	login = strings.Trim(login, " ")
	if len(login) < 3 || len(login) > 32 {
		return "", fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidLogin)
	}
	for i := range len(login) {
		c := login[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return "", fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidLogin)
		}
	}
	return strings.ToLower(login), nil
}

func validateAuthPassword(password string) error {
	if len(password) < 8 || len(password) > 128 {
		return fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidPassword)
	}
	for i := range len(password) {
		if password[i] < 33 || password[i] > 126 {
			return fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidPassword)
		}
	}
	return nil
}
