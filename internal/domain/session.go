package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	TokenHash []byte
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}
