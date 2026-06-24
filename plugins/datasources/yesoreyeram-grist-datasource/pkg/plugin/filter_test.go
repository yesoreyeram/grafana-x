package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Op: op, Value: value}
}

func groupRef(connector string, children ...FilterNode) *FilterNode {
	return &FilterNode{Kind: "group", Connector: connector, Children: children}
}

// ---------------------------------------------------------------------------
// simpleGristFilter (records endpoint membership filter)
// ---------------------------------------------------------------------------

func TestSimpleGristFilter_Nil(t *testing.T) {
	s, ok := simpleGristFilter(nil)
	require.True(t, ok)
	require.Equal(t, "", s)
}

func TestSimpleGristFilter_SingleEq(t *testing.T) {
	s, ok := simpleGristFilter(groupRef("and", cond("Plan", "eq", "pro")))
	require.True(t, ok)
	require.JSONEq(t, `{"Plan":["pro"]}`, s)
}

func TestSimpleGristFilter_InMembership(t *testing.T) {
	s, ok := simpleGristFilter(groupRef("and", cond("Plan", "in", "pro, team ,free")))
	require.True(t, ok)
	require.JSONEq(t, `{"Plan":["pro","team","free"]}`, s)
}

func TestSimpleGristFilter_MultipleColumnsAnd(t *testing.T) {
	s, ok := simpleGristFilter(groupRef("and", cond("A", "eq", "1"), cond("B", "eq", "2")))
	require.True(t, ok)
	require.JSONEq(t, `{"A":["1"],"B":["2"]}`, s)
}

func TestSimpleGristFilter_FallsBackToSQL(t *testing.T) {
	cases := []*FilterNode{
		groupRef("and", cond("Age", "gt", "30")),                                     // rich operator
		groupRef("or", cond("A", "eq", "1"), cond("B", "eq", "2")),                   // OR across columns
		groupRef("and", cond("A", "eq", "1"), *groupRef("or", cond("B", "eq", "2"))), // nested group
		groupRef("and", cond("A", "eq", "1"), cond("A", "eq", "2")),                  // repeated column
		groupRef("and", cond("Name", "contains", "x")),                               // contains
		groupRef("and", cond("Name", "neq", "x")),                                    // neq
	}
	for i, root := range cases {
		_, ok := simpleGristFilter(root)
		require.False(t, ok, "case %d should require SQL", i)
	}
}

func TestSimpleGristFilter_SkipsEmptyConditions(t *testing.T) {
	s, ok := simpleGristFilter(groupRef("and", cond("", "eq", "x"), cond("B", "eq", "2")))
	require.True(t, ok)
	require.JSONEq(t, `{"B":["2"]}`, s)
}

func TestSimpleGristFilter_NoEffectiveConditions(t *testing.T) {
	s, ok := simpleGristFilter(groupRef("and", cond("A", "eq", "")))
	require.True(t, ok)
	require.Equal(t, "", s)
}

// ---------------------------------------------------------------------------
// BuildWhere (parameterized SQL WHERE)
// ---------------------------------------------------------------------------

func TestBuildWhere_Nil(t *testing.T) {
	where, args := BuildWhere(nil)
	require.Equal(t, "", where)
	require.Nil(t, args)
}

func TestBuildWhere_Operators(t *testing.T) {
	cases := []struct {
		op        string
		value     string
		wantWhere string
		wantArgs  []any
	}{
		{"eq", "pro", `"Plan" = ?`, []any{"pro"}},
		{"neq", "pro", `"Plan" <> ?`, []any{"pro"}},
		{"gt", "5", `"Plan" > ?`, []any{float64(5)}},
		{"gte", "5", `"Plan" >= ?`, []any{float64(5)}},
		{"lt", "5", `"Plan" < ?`, []any{float64(5)}},
		{"lte", "5", `"Plan" <= ?`, []any{float64(5)}},
		{"contains", "ab", `"Plan" LIKE ?`, []any{"%ab%"}},
		{"not_contains", "ab", `("Plan" NOT LIKE ? OR "Plan" IS NULL)`, []any{"%ab%"}},
		{"empty", "", `"Plan" IS NULL`, []any{}},
		{"not_empty", "", `"Plan" IS NOT NULL`, []any{}},
	}
	for _, tc := range cases {
		where, args := BuildWhere(groupRef("and", cond("Plan", tc.op, tc.value)))
		require.Equal(t, tc.wantWhere, where, "op=%s", tc.op)
		require.Equal(t, tc.wantArgs, args, "op=%s", tc.op)
	}
}

func TestBuildWhere_InList(t *testing.T) {
	where, args := BuildWhere(groupRef("and", cond("Status", "in", "open, closed")))
	require.Equal(t, `"Status" IN (?, ?)`, where)
	require.Equal(t, []any{"open", "closed"}, args)
}

func TestBuildWhere_AndOfConditions(t *testing.T) {
	where, args := BuildWhere(groupRef("and", cond("A", "eq", "1"), cond("B", "gt", "2")))
	require.Equal(t, `("A" = ? AND "B" > ?)`, where)
	require.Equal(t, []any{"1", float64(2)}, args)
}

func TestBuildWhere_NestedOrGroup(t *testing.T) {
	root := groupRef("and",
		cond("A", "eq", "1"),
		*groupRef("or", cond("B", "eq", "2"), cond("C", "eq", "3")),
	)
	where, args := BuildWhere(root)
	require.Equal(t, `("A" = ? AND ("B" = ? OR "C" = ?))`, where)
	require.Equal(t, []any{"1", "2", "3"}, args)
}

func TestBuildWhere_SkipsConditionWithoutField(t *testing.T) {
	where, args := BuildWhere(groupRef("and", cond("", "eq", "x"), cond("B", "eq", "2")))
	require.Equal(t, `"B" = ?`, where)
	require.Equal(t, []any{"2"}, args)
}

func TestIdentifier_StripsEmbeddedQuotes(t *testing.T) {
	require.Equal(t, `"Name"`, identifier(`Na"me`))
	require.Equal(t, `"Age"`, identifier("Age"))
}

func TestSortCSV(t *testing.T) {
	require.Equal(t, "", sortCSV(nil))
	require.Equal(t, "Name,-Age", sortCSV([]SortItem{
		{Field: "Name", Direction: "asc"},
		{Field: "Age", Direction: "desc"},
	}))
	require.Equal(t, "Name", sortCSV([]SortItem{{Field: "", Direction: "asc"}, {Field: "Name"}}))
}
