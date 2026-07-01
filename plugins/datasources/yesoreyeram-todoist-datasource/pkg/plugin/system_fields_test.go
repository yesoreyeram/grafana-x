package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropSystemFields(t *testing.T) {
	records := []map[string]any{
		{"assigned_by_uid": "x", "_internal": 1, "name": "Alice", "value": 42},
	}

	out := dropSystemFields(records)
	require.Len(t, out, 1)
	// system + underscore-prefixed columns dropped.
	_, hasSys := out[0]["assigned_by_uid"]
	require.False(t, hasSys)
	_, hasUnderscore := out[0]["_internal"]
	require.False(t, hasUnderscore)
	// user data preserved.
	require.Equal(t, "Alice", out[0]["name"])
	require.Equal(t, 42, out[0]["value"])
}
