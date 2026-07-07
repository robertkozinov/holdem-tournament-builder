package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartPauseResume(t *testing.T) {
	startedAt := time.Date(2026, 7, 7, 20, 0, 0, 0, time.UTC)
	pausedAt := startedAt.Add(15 * time.Minute)
	resumedAt := pausedAt.Add(10 * time.Minute)

	tr := &Tournament{Status: StatusCreated}

	err := tr.Start(startedAt)
	assert.NoError(t, err)
	assert.Equal(t, StatusRunning, tr.Status)
	assert.Equal(t, startedAt, tr.LevelStartedAt)

	err = tr.Start(startedAt)
	assert.ErrorIs(t, err, ErrIncorrectStatus)

	err = tr.Pause(pausedAt)
	assert.NoError(t, err)
	assert.Equal(t, StatusPaused, tr.Status)
	assert.Equal(t, pausedAt, tr.PausedAt)

	err = tr.Pause(pausedAt)
	assert.ErrorIs(t, err, ErrIncorrectStatus)

	err = tr.Resume(resumedAt)
	assert.NoError(t, err)
	assert.Equal(t, StatusRunning, tr.Status)
	assert.Equal(t, startedAt.Add(10*time.Minute), tr.LevelStartedAt)

	err = tr.Resume(resumedAt)
	assert.ErrorIs(t, err, ErrIncorrectStatus)
}

func TestLevelUp(t *testing.T) {
	now := time.Date(2026, 7, 7, 21, 0, 0, 0, time.UTC)
	oldStartedAt := now.Add(-20 * time.Minute)

	tests := []struct {
		name             string
		status           Status
		currentLevel     int
		wantCurrentLevel int
		wantStartedAt    time.Time
		wantErr          error
	}{
		{
			name:             "next level",
			status:           StatusRunning,
			currentLevel:     0,
			wantCurrentLevel: 1,
			wantStartedAt:    now,
			wantErr:          nil,
		},
		{
			name:             "max level",
			status:           StatusRunning,
			currentLevel:     1,
			wantCurrentLevel: 1,
			wantStartedAt:    oldStartedAt,
			wantErr:          ErrMaxBlindLevel,
		},
		{
			name:             "incorrect status",
			status:           StatusPaused,
			currentLevel:     0,
			wantCurrentLevel: 0,
			wantStartedAt:    oldStartedAt,
			wantErr:          ErrIncorrectStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Tournament{
				Status:       tt.status,
				CurrentLevel: tt.currentLevel,
				BlindStructure: []BlindLevel{
					{SmallBlind: 10, BigBlind: 20, Duration: 20 * time.Minute},
					{SmallBlind: 20, BigBlind: 40, Duration: 20 * time.Minute},
				},
				LevelStartedAt: oldStartedAt,
			}
			err := tr.LevelUp(now)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantCurrentLevel, tr.CurrentLevel)
			assert.Equal(t, tt.wantStartedAt, tr.LevelStartedAt)
		})
	}
}

func TestBlindLevels(t *testing.T) {
	tr := &Tournament{
		CurrentLevel: 1,
		BlindStructure: []BlindLevel{
			{SmallBlind: 10, BigBlind: 20, Duration: 20 * time.Minute},
			{SmallBlind: 20, BigBlind: 40, Duration: 20 * time.Minute},
			{SmallBlind: 40, BigBlind: 80, Duration: 20 * time.Minute},
		},
	}

	current, err := tr.CurrentBlindLevel()
	assert.NoError(t, err)
	assert.Equal(t, BlindLevel{SmallBlind: 20, BigBlind: 40, Duration: 20 * time.Minute}, current)

	next, ok := tr.NextBlindLevel()
	assert.True(t, ok)
	assert.Equal(t, BlindLevel{SmallBlind: 40, BigBlind: 80, Duration: 20 * time.Minute}, next)

	tr.CurrentLevel = 2
	_, ok = tr.NextBlindLevel()
	assert.False(t, ok)

	tr.CurrentLevel = 3
	_, err = tr.CurrentBlindLevel()
	assert.ErrorIs(t, err, ErrMaxBlindLevel)
}

func TestTimeLeftInLevel(t *testing.T) {
	startedAt := time.Date(2026, 7, 7, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		status   Status
		now      time.Time
		pausedAt time.Time
		want     time.Duration
		wantErr  error
	}{
		{
			name:    "created tournament",
			status:  StatusCreated,
			now:     startedAt,
			want:    20 * time.Minute,
			wantErr: nil,
		},
		{
			name:    "running tournament",
			status:  StatusRunning,
			now:     startedAt.Add(5 * time.Minute),
			want:    15 * time.Minute,
			wantErr: nil,
		},
		{
			name:     "paused tournament",
			status:   StatusPaused,
			now:      startedAt.Add(15 * time.Minute),
			pausedAt: startedAt.Add(7 * time.Minute),
			want:     13 * time.Minute,
			wantErr:  nil,
		},
		{
			name:    "expired level",
			status:  StatusRunning,
			now:     startedAt.Add(25 * time.Minute),
			want:    0,
			wantErr: nil,
		},
		{
			name:    "finished tournament",
			status:  StatusFinished,
			now:     startedAt,
			want:    0,
			wantErr: ErrIncorrectStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Tournament{
				Status:         tt.status,
				CurrentLevel:   0,
				LevelStartedAt: startedAt,
				PausedAt:       tt.pausedAt,
				BlindStructure: []BlindLevel{
					{SmallBlind: 10, BigBlind: 20, Duration: 20 * time.Minute},
				},
			}
			got, err := tr.TimeLeftInLevel(tt.now)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAdvanceLevelIfNeeded(t *testing.T) {
	startedAt := time.Date(2026, 7, 7, 20, 0, 0, 0, time.UTC)

	tr := &Tournament{
		Status:         StatusRunning,
		CurrentLevel:   0,
		LevelStartedAt: startedAt,
		BlindStructure: []BlindLevel{
			{SmallBlind: 10, BigBlind: 20, Duration: 10 * time.Minute},
			{SmallBlind: 20, BigBlind: 40, Duration: 20 * time.Minute},
			{SmallBlind: 40, BigBlind: 80, Duration: 30 * time.Minute},
		},
	}

	err := tr.AdvanceLevelIfNeeded(startedAt.Add(35 * time.Minute))
	assert.NoError(t, err)
	assert.Equal(t, 2, tr.CurrentLevel)
	assert.Equal(t, startedAt.Add(30*time.Minute), tr.LevelStartedAt)

	err = tr.AdvanceLevelIfNeeded(startedAt.Add(2 * time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 2, tr.CurrentLevel)

	tr.Status = StatusPaused
	err = tr.AdvanceLevelIfNeeded(startedAt.Add(3 * time.Hour))
	assert.ErrorIs(t, err, ErrIncorrectStatus)
}

func TestAddRebuy(t *testing.T) {
	tests := []struct {
		name              string
		status            Status
		players           []string
		rebuyRules        RebuyRules
		currentLevel      int
		playerName        string
		wantContributions map[string]int64
		wantErr           error
	}{
		{
			name:              "rebuy allowed",
			status:            StatusRunning,
			players:           []string{"A", "B"},
			rebuyRules:        RebuyRules{Allowed: true, MaxLevel: 3},
			currentLevel:      1,
			playerName:        "A",
			wantContributions: map[string]int64{"A": 2000, "B": 1000},
			wantErr:           nil,
		},
		{
			name:              "player not found",
			status:            StatusRunning,
			players:           []string{"A", "B"},
			rebuyRules:        RebuyRules{Allowed: true, MaxLevel: 3},
			currentLevel:      1,
			playerName:        "C",
			wantContributions: map[string]int64{"A": 1000, "B": 1000},
			wantErr:           ErrPlayerNotFound,
		},
		{
			name:              "rebuy disabled",
			status:            StatusRunning,
			players:           []string{"A", "B"},
			rebuyRules:        RebuyRules{Allowed: false, MaxLevel: 3},
			currentLevel:      1,
			playerName:        "A",
			wantContributions: map[string]int64{"A": 1000, "B": 1000},
			wantErr:           ErrRebuyNotAllowed,
		},
		{
			name:              "rebuy after max level",
			status:            StatusRunning,
			players:           []string{"A", "B"},
			rebuyRules:        RebuyRules{Allowed: true, MaxLevel: 3},
			currentLevel:      4,
			playerName:        "A",
			wantContributions: map[string]int64{"A": 1000, "B": 1000},
			wantErr:           ErrRebuyNotAllowed,
		},
		{
			name:              "incorrect status",
			status:            StatusPaused,
			players:           []string{"A", "B"},
			rebuyRules:        RebuyRules{Allowed: true, MaxLevel: 3},
			currentLevel:      1,
			playerName:        "A",
			wantContributions: map[string]int64{"A": 1000, "B": 1000},
			wantErr:           ErrIncorrectStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := testRuntimeTournament(tt.players)
			tr.Contributions = map[string]int64{"A": 1000, "B": 1000}
			tr.RebuyRules = tt.rebuyRules
			tr.CurrentLevel = tt.currentLevel
			tr.Status = tt.status

			err := tr.AddRebuy(tt.playerName)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantContributions, tr.Contributions)
		})
	}
}

func TestKnockout(t *testing.T) {
	tests := []struct {
		name        string
		players     []string
		knockout    string
		wantResults []Result
		wantPlayers []string
		wantErr     error
	}{
		{
			name:        "knockout non-prize place",
			players:     []string{"A", "B", "C", "D"},
			knockout:    "D",
			wantResults: []Result{{Name: "D", Place: 4, Prize: 0}},
			wantPlayers: []string{"A", "B", "C"},
			wantErr:     nil,
		},
		{
			name:        "knockout prize place",
			players:     []string{"A", "B", "C"},
			knockout:    "C",
			wantResults: []Result{{Name: "C", Place: 3, Prize: 1000}},
			wantPlayers: []string{"A", "B"},
			wantErr:     nil,
		},
		{
			name:        "player not found",
			players:     []string{"A", "B", "C"},
			knockout:    "D",
			wantResults: nil,
			wantPlayers: []string{"A", "B", "C"},
			wantErr:     ErrPlayerNotFound,
		},
		{
			name:        "last player",
			players:     []string{"A"},
			knockout:    "A",
			wantResults: nil,
			wantPlayers: []string{"A"},
			wantErr:     ErrCantKnockout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := testRuntimeTournament(tt.players)
			err := tr.Knockout(tt.knockout)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.ElementsMatch(t, tt.wantResults, tr.Results)
			assert.Equal(t, tt.wantPlayers, tr.Players)
		})
	}
}

func TestFinish(t *testing.T) {
	tests := []struct {
		name        string
		players     []string
		wantStatus  Status
		wantResults []Result
		wantErr     error
	}{
		{
			name:        "finish tournament",
			players:     []string{"A"},
			wantStatus:  StatusFinished,
			wantResults: []Result{{Name: "A", Place: 1, Prize: 1000}},
			wantErr:     nil,
		},
		{
			name:        "cant finish with more than one player",
			players:     []string{"A", "B"},
			wantStatus:  StatusRunning,
			wantResults: nil,
			wantErr:     ErrCantFinish,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := testRuntimeTournament(tt.players)
			err := tr.Finish()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				err = tr.Finish()
				assert.ErrorIs(t, err, ErrIncorrectStatus)
			}
			assert.Equal(t, tt.wantStatus, tr.Status)
			assert.ElementsMatch(t, tt.wantResults, tr.Results)
		})
	}
}

func testRuntimeTournament(players []string) *Tournament {
	return &Tournament{
		Players:     players,
		BuyInAmount: 1000,
		PayoutSpots: []PayoutSpot{
			{Place: 1, Kind: PayoutRemainder},
			{Place: 2, Kind: PayoutFixed, BuyInsValue: 2},
			{Place: 3, Kind: PayoutFixed, BuyInsValue: 1},
		},
		Contributions: map[string]int64{
			"A": 1000,
			"B": 1000,
			"C": 1000,
			"D": 1000,
		},
		Status: StatusRunning,
	}
}
