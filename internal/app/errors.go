package app

import "errors"

var (
	ErrInvalidTournamentID = errors.New("invalid tournament id")
	ErrTournamentNotFound  = errors.New("tournament not found")
	ErrValidation          = errors.New("validation error")
)
