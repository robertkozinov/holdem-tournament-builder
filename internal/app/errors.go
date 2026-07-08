package app

import "errors"

var (
	ErrInvalidTournamentID = errors.New("tournament id must be greater than zero")
	ErrTournamentNotFound  = errors.New("tournament not found")
	ErrInvalidPlayerName   = errors.New("player name cant be empty")
)
