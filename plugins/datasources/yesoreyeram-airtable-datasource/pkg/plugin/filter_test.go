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

func TestBuildFormula_Nil(t *testing.T) {
	require.Equal(t, "", BuildFormula(nil))
}

func TestBuildFormula_Empty(t *testing.T) {
	require.Equal(t, "", BuildFormula(group("and")))
}

func TestBuildFormula_SingleCondition_NoWrappingAnd(t *testing.T) {
	require.Equal(t, `{Plan} = "pro"`, BuildFormula(group("and", cond("Plan", "eq", "pro"))))
}

func TestBuildFormula_Operators(t *testing.T) {
	cases := []struct {
		op    string
		value string
		want  string
	}{
		{"eq", "x", `{F} = "x"`},
		{"neq", "x", `{F} != "x"`},
		{"gt", "5", `{F} > 5`},
		{"gte", "5", `{F} >= 5`},
		{"lt", "5", `{F} < 5`},
		{"lte", "5", `{F} <= 5`},
		{"gt", "abc", `{F} > "abc"`}, // non-numeric comparison is quoted
		{"contains", "ab", `FIND(LOWER("ab"), LOWER({F} & "")) > 0`},
		{"not_contains", "ab", `FIND(LOWER("ab"), LOWER({F} & "")) = 0`},
		{"empty", "", `{F} = BLANK()`},
		{"not_empty", "", `NOT({F} = BLANK())`},
		{"is_true", "", `{F}`},
		{"is_false", "", `NOT({F})`},
		{"unknown_op", "x", `{F} = "x"`}, // falls back to equality
	}
	for _, tc := range cases {
		got := BuildFormula(group("and", cond("F", tc.op, tc.value)))
		require.Equal(t, tc.want, got, "op=%s", tc.op)
	}
}

func TestBuildFormula_AndOr(t *testing.T) {
	got := BuildFormula(group("and", cond("A", "eq", "1"), cond("B", "eq", "2")))
	require.Equal(t, `AND({A} = "1", {B} = "2")`, got)

	got = BuildFormula(group("or", cond("A", "eq", "1"), cond("B", "eq", "2")))
	require.Equal(t, `OR({A} = "1", {B} = "2")`, got)
}

func TestBuildFormula_NestedGroups(t *testing.T) {
	root := group("and",
		cond("A", "eq", "1"),
		*group("or", cond("B", "eq", "2"), cond("C", "eq", "3")),
	)
	got := BuildFormula(root)
	require.Equal(t, `AND({A} = "1", OR({B} = "2", {C} = "3"))`, got)
}

func TestBuildFormula_SkipsConditionsWithoutField(t *testing.T) {
	got := BuildFormula(group("and", cond("", "eq", "x"), cond("B", "eq", "2")))
	require.Equal(t, `{B} = "2"`, got)
}

func TestBuildFormula_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("A", "eq", "1"), *group("and"))
	got := BuildFormula(root)
	require.Equal(t, `{A} = "1"`, got)
}

func TestBuildFormula_EscapesQuotes(t *testing.T) {
	got := BuildFormula(group("and", cond("Name", "eq", `a"b`)))
	require.Equal(t, `{Name} = "a\"b"`, got)
}

func TestBuildFormula_EscapesFieldBrace(t *testing.T) {
	got := BuildFormula(group("and", cond("we}ird", "eq", "x")))
	require.Equal(t, `{weird} = "x"`, got)
}

func TestIsNumeric(t *testing.T) {
	for _, s := range []string{"5", "-5", "5.5", "-5.25", "0"} {
		require.True(t, isNumeric(s), s)
	}
	for _, s := range []string{"", "abc", "5.5.5", "-", ".", "5a", "1e3"} {
		require.False(t, isNumeric(s), s)
	}
}
