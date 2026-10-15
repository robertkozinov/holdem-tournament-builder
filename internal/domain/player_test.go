package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePlayerName(t *testing.T) {
	for _, tc := range []struct{ input, display, normalized string }{
		{"  Влад\tФед  ", "Влад Фед", "влад фед"},
		{"ВЛАД", "ВЛАД", "влад"},
		{"Артём\u00a0Поп", "Артём Поп", "артём поп"},
		{"Player_7%", "Player_7%", "player_7%"},
		{strings.Repeat("Я", 100), strings.Repeat("Я", 100), strings.Repeat("я", 100)},
	} {
		t.Run(tc.input, func(t *testing.T) {
			display, normalized, err := NormalizePlayerName(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.display, display)
			assert.Equal(t, tc.normalized, normalized)
		})
	}
	for _, name := range []string{"", " \n\t", strings.Repeat("Я", 101), "Имя\x00", "Имя\x1b", string([]byte{0xff})} {
		t.Run("invalid "+name, func(t *testing.T) {
			display, normalized, err := NormalizePlayerName(name)
			require.ErrorIs(t, err, ErrInvalidPlayerName)
			assert.Empty(t, display)
			assert.Empty(t, normalized)
		})
	}
}
