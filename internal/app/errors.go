package app

import "errors"

var (
	ErrInvalidTournamentID   = errors.New("invalid tournament id")
	ErrTournamentNotFound    = errors.New("tournament not found")
	ErrInvalidPlayerName     = errors.New("player name cant be empty")
	ErrInvalidPayoutMode     = errors.New("invalid payout mode")
	ErrValidation            = errors.New("validation error")
	ErrInvalidUsername       = errors.New("invalid username")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUnauthenticated       = errors.New("unauthenticated")
)
