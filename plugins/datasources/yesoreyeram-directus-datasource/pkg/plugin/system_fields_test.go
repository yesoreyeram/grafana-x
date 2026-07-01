package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropSystemFields(t *testing.T) {
	records := []map[string]any{
		{"user_created": "x", "_internal": 1, "name": "Alice", "value": 42},
	}

	out := dropSystemFields(records)
	require.Len(t, out, 1)
	// system + underscore-prefixed columns dropped.
	_, hasSys := out[0]["user_created"]
	require.False(t, hasSys)
	_, hasUnderscore := out[0]["_internal"]
	require.False(t, hasUnderscore)
	// user data preserved.
	require.Equal(t, "Alice", out[0]["name"])
	require.Equal(t, 42, out[0]["value"])
}
