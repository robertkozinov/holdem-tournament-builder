package domain

import "errors"

var ErrEmptyName = errors.New("tournament name must not be empty")
var ErrNotEnoughPlayers = errors.New("must be at least two players")
var ErrDuplicatePlayer = errors.New("names must not be duplicated")
var ErrIncorrectBuyInAmount = errors.New("buy in cant be negative")
var ErrEmptyChipSet = errors.New("must be at least one chip denomination")
var ErrIncorrectChipSet = errors.New("chip denominations must have positive value and count")
var ErrDuplicateChipDenomination = errors.New("chip denominations must not be duplicated")
var ErrIncorrectDuration = errors.New("duration cant be zero or negative")
var ErrIncorrectRebuyRules = errors.New("incorrect rebuy rules")

var ErrNotEnoughLevels = errors.New("duration is too short for chosen level length")
var ErrSmallStack = errors.New("stack is  too small")

var ErrMaxBlindLevel = errors.New("max blind level reached")
var ErrPlayerNotFound = errors.New("player not found")
var ErrCantKnockout = errors.New("cant knockout last player")
var ErrCantFinish = errors.New("tournament can be finished only with one player left")
var ErrRebuyNotAllowed = errors.New("rebuy not allowed")

var ErrIncorrectStatus = errors.New("incorrect status")

var ErrTooManyPayoutPlaces = errors.New("payout places exceed number of players")
var ErrDuplicatePayoutPlace = errors.New("duplicate payout place")
var ErrInvalidBuyInsValue = errors.New("fixed payout must have positive buy-ins value")
var ErrInvalidPayoutKind = errors.New("invalid payout kind")
var ErrRemainderRequired = errors.New("exactly one remainder payout required")
var ErrGapInPayoutPlaces = errors.New("payout places must be sequential without gaps")
var ErrRemainderNotPositive = errors.New("fixed payouts exceed pot, remainder would be non-positive")
