package frontend

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesContainsInputFormatters(t *testing.T) {
	_, err := Files.ReadFile("js/input-formatters.mjs")
	require.NoError(t, err)
}
