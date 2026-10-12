package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Login        string
	PasswordHash string `json:"-"`
	CreatedAt    time.Time
}
