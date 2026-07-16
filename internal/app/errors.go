package app

import "errors"

var (
	ErrInvalidTournamentID = errors.New("invalid tournament id")
	ErrTournamentNotFound  = errors.New("tournament not found")
	ErrInvalidPlayerName   = errors.New("player name cant be empty")
)
