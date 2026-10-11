package domain

import "errors"

var ErrEmptyName = errors.New("tournament name must not be empty")
var ErrNotEnoughPlayers = errors.New("must be at least two players")
var ErrDuplicatePlayer = errors.New("names must not be duplicated")
var ErrIncorrectBuyInAmount = errors.New("buy in cant be negative")
var ErrEmptyChipSet = errors.New("must be at least one chip denomination")
var ErrIncorrectChipSet = errors.New("chip denominations must have positive value and count")
var ErrDuplicateChipValue = errors.New("chip denomination values must not be duplicated")
var ErrIncorrectDuration = errors.New("duration cant be zero or negative")

var ErrNotEnoughLevels = errors.New("duration is too short for chosen level length")
var ErrSmallStack = errors.New("stack is  too small")

var ErrPlayerNotFound = errors.New("player not found")

var ErrInvalidPlayerName = errors.New("player name must not be empty")
var ErrInvalidPlayerID = errors.New("player id must not be empty")
var ErrInvalidRebuyID = errors.New("rebuy id must not be empty")
var ErrDuplicateRebuy = errors.New("rebuy ids must not be duplicated")
var ErrPotOverflow = errors.New("pot exceeds supported amount")
var ErrChipSetOverflow = errors.New("chip set exceeds supported amount")
var ErrTooManyBlindLevels = errors.New("blind structure must not exceed 1000 levels")
var ErrInvalidSettlementMode = errors.New("settlement mode must be bank or p2p")
var ErrInvalidStyle = errors.New("invalid tournament style")
var ErrDuplicateResult = errors.New("player has more than one result")
var ErrInvalidResultPlace = errors.New("prize place must be within player count")
var ErrInvalidPrize = errors.New("prize must be positive")
var ErrPrizeSumMismatch = errors.New("sum of prizes must equal pot")

var ErrTooManyPayoutPlaces = errors.New("payout places exceed number of players")
var ErrDuplicatePayoutPlace = errors.New("duplicate payout place")
var ErrInvalidBuyInsValue = errors.New("fixed payout must have positive buy-ins value")
var ErrInvalidPayoutKind = errors.New("invalid payout kind")
var ErrRemainderRequired = errors.New("exactly one remainder payout required")
var ErrGapInPayoutPlaces = errors.New("payout places must be sequential without gaps")
var ErrRemainderNotPositive = errors.New("fixed payouts exceed pot, remainder would be non-positive")
