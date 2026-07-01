package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Op: op, Value: value}
}

func group(connector string, children ...FilterNode) *FilterNode {
	return &FilterNode{Kind: "group", Connector: connector, Children: children}
}

func TestBuildWhere_Nil(t *testing.T) {
	where, params := BuildWhere(nil)
	require.Equal(t, "", where)
	require.Nil(t, params)
}

func TestBuildWhere_Empty(t *testing.T) {
	where, params := BuildWhere(group("and"))
	require.Equal(t, "", where)
	require.Nil(t, params)
}

func TestBuildWhere_SingleCondition_NoWrap(t *testing.T) {
	where, params := BuildWhere(group("and", cond("Plan", "eq", "pro")))
	require.Equal(t, "`Plan` = ?", where)
	require.Equal(t, []any{"pro"}, params)
}

func TestBuildWhere_Operators(t *testing.T) {
	cases := []struct {
		op     string
		value  string
		where  string
		params []any
	}{
		{"eq", "x", "`F` = ?", []any{"x"}},
		{"neq", "x", "`F` <> ?", []any{"x"}},
		{"gt", "5", "`F` > ?", []any{float64(5)}},
		{"gte", "5", "`F` >= ?", []any{float64(5)}},
		{"lt", "5", "`F` < ?", []any{float64(5)}},
		{"lte", "5", "`F` <= ?", []any{float64(5)}},
		{"gt", "abc", "`F` > ?", []any{"abc"}}, // non-numeric comparison stays string
		{"contains", "ab", "`F` ILIKE ?", []any{"%ab%"}},
		{"not_contains", "ab", "NOT (`F` ILIKE ?)", []any{"%ab%"}},
		{"empty", "", "`F` IS NULL", nil},
		{"is_empty", "", "`F` IS NULL", nil},
		{"not_empty", "", "`F` IS NOT NULL", nil},
		{"is_not_empty", "", "`F` IS NOT NULL", nil},
		{"is_true", "", "`F` IS TRUE", nil},
		{"is_false", "", "`F` IS NOT TRUE", nil},
		{"unknown_op", "x", "`F` = ?", []any{"x"}}, // falls back to equality
	}
	for _, tc := range cases {
		where, params := BuildWhere(group("and", cond("F", tc.op, tc.value)))
		require.Equal(t, tc.where, where, "op=%s", tc.op)
		if tc.params == nil {
			require.Empty(t, params, "op=%s", tc.op)
		} else {
			require.Equal(t, tc.params, params, "op=%s", tc.op)
		}
	}
}

func TestBuildWhere_InList(t *testing.T) {
	where, params := BuildWhere(group("and", cond("City", "in", "Paris, London, Berlin")))
	require.Equal(t, "`City` IN (?, ?, ?)", where)
	require.Equal(t, []any{"Paris", "London", "Berlin"}, params)
}

func TestBuildWhere_NotInList(t *testing.T) {
	where, params := BuildWhere(group("and", cond("City", "not_in", "Paris,London")))
	require.Equal(t, "`City` NOT IN (?, ?)", where)
	require.Equal(t, []any{"Paris", "London"}, params)
}

func TestBuildWhere_AndOr(t *testing.T) {
	where, params := BuildWhere(group("and", cond("A", "eq", "1"), cond("B", "eq", "2")))
	require.Equal(t, "(`A` = ? AND `B` = ?)", where)
	require.Equal(t, []any{"1", "2"}, params)

	where, params = BuildWhere(group("or", cond("A", "eq", "1"), cond("B", "eq", "2")))
	require.Equal(t, "(`A` = ? OR `B` = ?)", where)
	require.Equal(t, []any{"1", "2"}, params)
}

func TestBuildWhere_NestedGroups(t *testing.T) {
	root := group("and",
		cond("A", "eq", "1"),
		*group("or", cond("B", "eq", "2"), cond("C", "eq", "3")),
	)
	where, params := BuildWhere(root)
	require.Equal(t, "(`A` = ? AND (`B` = ? OR `C` = ?))", where)
	require.Equal(t, []any{"1", "2", "3"}, params)
}

func TestBuildWhere_SkipsConditionsWithoutField(t *testing.T) {
	where, params := BuildWhere(group("and", cond("", "eq", "x"), cond("B", "eq", "2")))
	require.Equal(t, "`B` = ?", where)
	require.Equal(t, []any{"2"}, params)
}

func TestBuildWhere_SkipsEmptyValues(t *testing.T) {
	// eq with an empty value is dropped (not all rows match accidentally).
	where, params := BuildWhere(group("and", cond("A", "eq", ""), cond("B", "eq", "2")))
	require.Equal(t, "`B` = ?", where)
	require.Equal(t, []any{"2"}, params)
}

func TestBuildWhere_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("A", "eq", "1"), *group("and"))
	where, params := BuildWhere(root)
	require.Equal(t, "`A` = ?", where)
	require.Equal(t, []any{"1"}, params)
}

func TestIdentifier_StripsBackticks(t *testing.T) {
	require.Equal(t, "`weird`", identifier("we`ird"))
}

func TestBuildWhere_ValuesAreParameterizedNotInlined(t *testing.T) {
	// A malicious value never reaches the SQL string; it is bound as a parameter.
	where, params := BuildWhere(group("and", cond("Name", "eq", `'; DROP TABLE t; --`)))
	require.Equal(t, "`Name` = ?", where)
	require.Equal(t, []any{`'; DROP TABLE t; --`}, params)
}
