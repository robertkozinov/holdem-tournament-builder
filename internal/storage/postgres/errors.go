package postgres

import "errors"

var errUnknownTournamentStatus = errors.New("unknown tournament status")
var errNilTournament = errors.New("tournament is nil")
