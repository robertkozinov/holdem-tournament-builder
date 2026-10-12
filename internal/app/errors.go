package app

import "errors"

var (
	ErrInvalidTournamentID = errors.New("invalid tournament id")
	ErrTournamentNotFound  = errors.New("tournament not found")
	ErrValidation          = errors.New("validation error")
	ErrUnauthenticated     = errors.New("authentication required")
	ErrInvalidCredentials  = errors.New("invalid login or password")
	ErrLoginTaken          = errors.New("login is already taken")
	ErrUserNotFound        = errors.New("user not found")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionUnavailable  = errors.New("session storage unavailable")
	ErrInvalidLogin        = errors.New("invalid login")
	ErrInvalidPassword     = errors.New("invalid password")
)
