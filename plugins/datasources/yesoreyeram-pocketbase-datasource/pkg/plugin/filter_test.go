package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(attribute, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Attribute: attribute, Op: op, Value: value}
}

func group(connector string, children ...FilterNode) *FilterNode {
	return &FilterNode{Kind: "group", Connector: connector, Children: children}
}

func TestBuildFilter_Nil(t *testing.T) {
	require.Equal(t, "", BuildFilter(nil))
}

func TestBuildFilter_Empty(t *testing.T) {
	require.Equal(t, "", BuildFilter(group("and")))
}

func TestBuildFilter_SingleCondition(t *testing.T) {
	require.Equal(t, `status = 'active'`, BuildFilter(group("and", cond("status", "equal", "active"))))
}

func TestBuildFilter_Operators(t *testing.T) {
	cases := []struct {
		op    string
		value string
		want  string
	}{
		{"equal", "x", `F = 'x'`},
		{"notEqual", "x", `F != 'x'`},
		{"greaterThan", "5", `F > 5`},
		{"greaterThanEqual", "5", `F >= 5`},
		{"lessThan", "5", `F < 5`},
		{"lessThanEqual", "5", `F <= 5`},
		{"equal", "5.5", `F = 5.5`},
		{"equal", "true", `F = true`},
		{"equal", "false", `F = false`},
		{"contains", "ab", `F ~ 'ab'`},
		{"notContains", "ab", `F !~ 'ab'`},
		{"isNull", "", `F = null`},
		{"isNotNull", "", `F != null`},
		{"unknown_op", "x", `F = 'x'`}, // falls back to equality
		// String operators must keep numeric-looking values as strings.
		{"contains", "5", `F ~ '5'`},
		// Equality with a literal null keyword.
		{"equal", "null", `F = null`},
	}
	for _, tc := range cases {
		got := BuildFilter(group("and", cond("F", tc.op, tc.value)))
		require.Equal(t, tc.want, got, "op=%s value=%s", tc.op, tc.value)
	}
}

func TestBuildFilter_AndJoinsWithDoubleAmp(t *testing.T) {
	got := BuildFilter(group("and", cond("A", "equal", "1"), cond("B", "equal", "2")))
	require.Equal(t, `A = 1 && B = 2`, got)
}

func TestBuildFilter_OrJoinsWithDoublePipe(t *testing.T) {
	got := BuildFilter(group("or", cond("A", "equal", "1"), cond("B", "equal", "2")))
	require.Equal(t, `A = 1 || B = 2`, got)
}

func TestBuildFilter_NestedGroupWrappedInParens(t *testing.T) {
	root := group("and",
		cond("A", "equal", "1"),
		*group("or", cond("B", "equal", "2"), cond("C", "equal", "3")),
	)
	require.Equal(t, `A = 1 && (B = 2 || C = 3)`, BuildFilter(root))
}

func TestBuildFilter_SkipsConditionsWithoutAttribute(t *testing.T) {
	got := BuildFilter(group("and", cond("", "equal", "x"), cond("B", "equal", "2")))
	require.Equal(t, `B = 2`, got)
}

func TestBuildFilter_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("A", "equal", "1"), *group("and"))
	require.Equal(t, `A = 1`, BuildFilter(root))
}

func TestQuote_EscapesQuotesAndBackslashes(t *testing.T) {
	require.Equal(t, `'plain'`, quote("plain"))
	require.Equal(t, `'o\'brien'`, quote("o'brien"))
	require.Equal(t, `'a\\b'`, quote(`a\b`))
}

func TestFormatValue(t *testing.T) {
	// text-match ops always quote.
	require.Equal(t, `'5'`, formatValue("contains", "5"))
	require.Equal(t, `'true'`, formatValue("notContains", "true"))
	// equality/comparison coerce literals.
	require.Equal(t, `true`, formatValue("equal", "true"))
	require.Equal(t, `false`, formatValue("equal", "false"))
	require.Equal(t, `null`, formatValue("equal", "null"))
	require.Equal(t, `5`, formatValue("equal", "5"))
	require.Equal(t, `-5`, formatValue("greaterThan", "-5"))
	require.Equal(t, `5.5`, formatValue("equal", "5.5"))
	require.Equal(t, `'hello'`, formatValue("equal", "hello"))
	require.Equal(t, `'5a'`, formatValue("equal", "5a"))
}

func TestSortParam(t *testing.T) {
	got := sortParam([]SortItem{
		{Attribute: "age", Direction: "desc"},
		{Attribute: "name", Direction: "asc"},
		{Attribute: "", Direction: "asc"}, // skipped
	})
	require.Equal(t, "-age,name", got)
}

func TestFieldsParam(t *testing.T) {
	// Without hiding system fields, the identity fields are appended (deduped).
	q, ok := fieldsParam("name, age", false)
	require.True(t, ok)
	require.Equal(t, "name,age,id,created,updated", q)

	// id already present is not duplicated.
	q, ok = fieldsParam("id, name", false)
	require.True(t, ok)
	require.Equal(t, "id,name,created,updated", q)

	// When hiding system fields, only the requested fields are projected.
	q, ok = fieldsParam("name, age", true)
	require.True(t, ok)
	require.Equal(t, "name,age", q)

	_, ok = fieldsParam("", false)
	require.False(t, ok)
	_, ok = fieldsParam("  ,  ", false)
	require.False(t, ok)
}
