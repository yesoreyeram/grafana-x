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

func TestBuildFilter_Nil(t *testing.T) {
	require.Nil(t, BuildFilter(nil))
}

func TestBuildFilter_EmptyGroup(t *testing.T) {
	require.Nil(t, BuildFilter(group("and")))
}

func TestBuildFilter_SingleCondition(t *testing.T) {
	f := BuildFilter(group("and", cond("status", "eq", "published")))
	require.NotNil(t, f)
	expected := map[string]any{"status": map[string]any{"_eq": "published"}}
	require.Equal(t, expected, f)
}

func TestBuildFilter_Operators(t *testing.T) {
	tests := []struct {
		op    string
		value string
		want  map[string]any
	}{
		{"eq", "x", map[string]any{"_eq": "x"}},
		{"neq", "x", map[string]any{"_neq": "x"}},
		{"gt", "5", map[string]any{"_gt": "5"}},
		{"gte", "5", map[string]any{"_gte": "5"}},
		{"lt", "5", map[string]any{"_lt": "5"}},
		{"lte", "5", map[string]any{"_lte": "5"}},
		{"contains", "ab", map[string]any{"_contains": "ab"}},
		{"ncontains", "ab", map[string]any{"_ncontains": "ab"}},
		{"icontains", "ab", map[string]any{"_icontains": "ab"}},
		{"startsWith", "ab", map[string]any{"_starts_with": "ab"}},
		{"endsWith", "ab", map[string]any{"_ends_with": "ab"}},
		{"in", "a,b", map[string]any{"_in": []string{"a", "b"}}},
		{"nin", "a,b", map[string]any{"_nin": []string{"a", "b"}}},
		{"between", "1,10", map[string]any{"_between": []string{"1", "10"}}},
		{"nbetween", "1,10", map[string]any{"_nbetween": []string{"1", "10"}}},
		{"null", "", map[string]any{"_null": true}},
		{"nnull", "", map[string]any{"_nnull": true}},
		{"empty", "", map[string]any{"_empty": true}},
		{"nempty", "", map[string]any{"_nempty": true}},
		{"not_empty", "", map[string]any{"_nempty": true}}, // backward-compat alias
	}
	for _, tt := range tests {
		f := BuildFilter(group("and", cond("F", tt.op, tt.value)))
		require.NotNil(t, f, "op=%s", tt.op)
		condMap, ok := f["F"].(map[string]any)
		require.True(t, ok, "op=%s", tt.op)
		require.Equal(t, tt.want, condMap, "op=%s", tt.op)
	}
}

func TestBuildFilter_BetweenRequiresTwoValues(t *testing.T) {
	require.Nil(t, BuildFilter(group("and", cond("price", "between", "5"))))
	require.Nil(t, BuildFilter(group("and", cond("price", "between", ""))))
	f := BuildFilter(group("and", cond("price", "between", "5,10")))
	require.Equal(t, map[string]any{"price": map[string]any{"_between": []string{"5", "10"}}}, f)
}

func TestBuildFilter_UnknownOpFallsBackToEq(t *testing.T) {
	f := BuildFilter(group("and", cond("a", "bogus", "1")))
	require.Equal(t, map[string]any{"a": map[string]any{"_eq": "1"}}, f)
}

func TestBuildFilter_And(t *testing.T) {
	f := BuildFilter(group("and", cond("a", "eq", "1"), cond("b", "eq", "2")))
	require.NotNil(t, f)
	andList, ok := f["_and"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, andList, 2)
}

func TestBuildFilter_Or(t *testing.T) {
	f := BuildFilter(group("or", cond("a", "eq", "1"), cond("b", "eq", "2")))
	require.NotNil(t, f)
	orList, ok := f["_or"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, orList, 2)
}

func TestBuildFilter_NestedGroups(t *testing.T) {
	root := group("and",
		cond("a", "eq", "1"),
		*group("or", cond("b", "eq", "2"), cond("c", "eq", "3")),
	)
	f := BuildFilter(root)
	require.NotNil(t, f)
	andList, ok := f["_and"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, andList, 2)
	_, isOr := andList[1]["_or"]
	require.True(t, isOr)
}

func TestBuildFilter_SkipsConditionsWithoutField(t *testing.T) {
	f := BuildFilter(group("and", cond("", "eq", "x"), cond("b", "eq", "2")))
	require.NotNil(t, f)
	condB, ok := f["b"]
	require.True(t, ok)
	require.Equal(t, map[string]any{"_eq": "2"}, condB)
}

func TestBuildFilter_SkipsConditionsWithEmptySingleValue(t *testing.T) {
	// An empty value for a single-value operator is dropped (so it is not sent
	// as a filter for the empty string).
	f := BuildFilter(group("and", cond("a", "eq", ""), cond("b", "eq", "2")))
	require.NotNil(t, f)
	_, hasA := f["a"]
	require.False(t, hasA)
	require.Equal(t, map[string]any{"_eq": "2"}, f["b"])
}

func TestBuildFilter_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("a", "eq", "1"), *group("and"))
	f := BuildFilter(root)
	require.NotNil(t, f)
	condA, ok := f["a"]
	require.True(t, ok)
	require.Equal(t, map[string]any{"_eq": "1"}, condA)
}

func TestParseListValue(t *testing.T) {
	require.Equal(t, []string{"a", "b", "c"}, parseListValue("a,b,c"))
	require.Equal(t, []string{"hello", "world"}, parseListValue(" hello , world "))
	require.Nil(t, parseListValue(""))
	require.Nil(t, parseListValue("   "))
}

func TestOperatorArity(t *testing.T) {
	require.Equal(t, "none", operatorArity("empty"))
	require.Equal(t, "none", operatorArity("null"))
	require.Equal(t, "list", operatorArity("in"))
	require.Equal(t, "between", operatorArity("between"))
	require.Equal(t, "single", operatorArity("eq"))
}
