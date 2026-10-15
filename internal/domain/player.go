package domain

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Player struct {
	ID             uuid.UUID
	Name           string
	NormalizedName string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func NormalizePlayerName(name string) (display, normalized string, err error) {
	if !utf8.ValidString(name) {
		return "", "", ErrInvalidPlayerName
	}
	for _, r := range name {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return "", "", ErrInvalidPlayerName
		}
	}
	display = strings.Join(strings.Fields(name), " ")
	if count := utf8.RuneCountInString(display); count < 1 || count > 100 {
		return "", "", ErrInvalidPlayerName
	}
	return display, strings.ToLower(display), nil
}
