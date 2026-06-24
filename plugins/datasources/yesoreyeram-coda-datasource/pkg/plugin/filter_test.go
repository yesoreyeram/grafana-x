package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildColumnQuery(t *testing.T) {
	cases := []struct {
		name   string
		column string
		value  string
		want   string
	}{
		{"empty column", "", "x", ""},
		{"empty value", "Plan", "", ""},
		{"name is quoted, string value quoted", "Plan", "pro", `"Plan":"pro"`},
		{"column id used as-is", "c-abc123", "pro", `c-abc123:"pro"`},
		{"numeric value emitted raw", "Age", "30", `"Age":30`},
		{"float value emitted raw", "Score", "9.5", `"Score":9.5`},
		{"boolean value emitted raw", "Active", "true", `"Active":true`},
		{"name with spaces quoted", "My Column", "groceries", `"My Column":"groceries"`},
		{"value with quotes is escaped", "Note", `say "hi"`, `"Note":"say \"hi\""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, buildColumnQuery(tc.column, tc.value))
		})
	}
}

func TestEffectiveFilterQuery_RawTakesPrecedence(t *testing.T) {
	q := QueryModel{
		Query:        `c-zzz:"custom"`,
		FilterColumn: "Plan",
		FilterValue:  "pro",
	}
	require.Equal(t, `c-zzz:"custom"`, effectiveFilterQuery(q))
}

func TestEffectiveFilterQuery_FallsBackToColumnFilter(t *testing.T) {
	q := QueryModel{FilterColumn: "Plan", FilterValue: "pro"}
	require.Equal(t, `"Plan":"pro"`, effectiveFilterQuery(q))
}

func TestEffectiveFilterQuery_EmptyWhenNoFilter(t *testing.T) {
	require.Equal(t, "", effectiveFilterQuery(QueryModel{}))
}

func TestIsColumnID(t *testing.T) {
	require.True(t, isColumnID("c-abc123"))
	require.False(t, isColumnID("Plan"))
	require.False(t, isColumnID(""))
}
