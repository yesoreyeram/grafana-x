package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Op: op, Value: value}
}

func condCat(field, category, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Category: category, Op: op, Value: value}
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
	f := BuildFilter(group("and", cond("stage", "eq", "Won")))
	require.NotNil(t, f)
	require.Equal(t, map[string]any{"stage": map[string]any{"$eq": "Won"}}, f)
}

func TestBuildFilter_NotEquals(t *testing.T) {
	f := BuildFilter(group("and", cond("stage", "neq", "Lost")))
	require.NotNil(t, f)
	expected := map[string]any{"$not": map[string]any{"stage": map[string]any{"$eq": "Lost"}}}
	require.Equal(t, expected, f)
}

func TestBuildFilter_Empty(t *testing.T) {
	f := BuildFilter(group("and", cond("email_addresses", "empty", "")))
	require.NotNil(t, f)
	expected := map[string]any{"$not": map[string]any{"email_addresses": map[string]any{"$not_empty": true}}}
	require.Equal(t, expected, f)
}

func TestBuildFilter_NotEmpty(t *testing.T) {
	f := BuildFilter(group("and", cond("email_addresses", "not_empty", "")))
	require.NotNil(t, f)
	expected := map[string]any{"email_addresses": map[string]any{"$not_empty": true}}
	require.Equal(t, expected, f)
}

func TestBuildFilter_StringOperators(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"contains", "$contains"},
		{"startsWith", "$starts_with"},
		{"endsWith", "$ends_with"},
	}
	for _, tt := range tests {
		f := BuildFilter(group("and", condCat("name", "text", tt.op, "Ada")))
		condMap, ok := f["name"].(map[string]any)
		require.True(t, ok, "op=%s", tt.op)
		require.Equal(t, map[string]any{tt.want: "Ada"}, condMap, "op=%s", tt.op)
	}
}

func TestBuildFilter_NumberComparisons(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"gt", "$gt"},
		{"gte", "$gte"},
		{"lt", "$lt"},
		{"lte", "$lte"},
	}
	for _, tt := range tests {
		f := BuildFilter(group("and", condCat("amount", "number", tt.op, "500")))
		condMap, ok := f["amount"].(map[string]any)
		require.True(t, ok, "op=%s", tt.op)
		require.Equal(t, map[string]any{tt.want: float64(500)}, condMap, "op=%s", tt.op)
	}
}

func TestBuildFilter_NumberEqualityCoercesValue(t *testing.T) {
	f := BuildFilter(group("and", condCat("amount", "number", "eq", "42")))
	require.Equal(t, map[string]any{"amount": map[string]any{"$eq": float64(42)}}, f)
}

func TestBuildFilter_BooleanCoercion(t *testing.T) {
	f := BuildFilter(group("and", condCat("is_active", "boolean", "eq", "true")))
	require.Equal(t, map[string]any{"is_active": map[string]any{"$eq": true}}, f)
}

func TestBuildFilter_DateStaysString(t *testing.T) {
	f := BuildFilter(group("and", condCat("close_date", "date", "gte", "2019-01-01")))
	require.Equal(t, map[string]any{"close_date": map[string]any{"$gte": "2019-01-01"}}, f)
}

func TestBuildFilter_In(t *testing.T) {
	f := BuildFilter(group("and", cond("record_id", "in", "a, b ,c")))
	require.NotNil(t, f)
	condMap, ok := f["record_id"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"$in": []any{"a", "b", "c"}}, condMap)
}

func TestBuildFilter_InEmptySkipped(t *testing.T) {
	f := BuildFilter(group("and", cond("record_id", "in", "   ")))
	require.Nil(t, f)
}

func TestBuildFilter_And(t *testing.T) {
	f := BuildFilter(group("and", cond("a", "eq", "1"), cond("b", "eq", "2")))
	require.NotNil(t, f)
	andList, ok := f["$and"].([]any)
	require.True(t, ok)
	require.Len(t, andList, 2)
}

func TestBuildFilter_Or(t *testing.T) {
	f := BuildFilter(group("or", cond("a", "eq", "1"), cond("b", "eq", "2")))
	require.NotNil(t, f)
	orList, ok := f["$or"].([]any)
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
	andList, ok := f["$and"].([]any)
	require.True(t, ok)
	require.Len(t, andList, 2)
	second, ok := andList[1].(map[string]any)
	require.True(t, ok)
	_, isOr := second["$or"]
	require.True(t, isOr)
}

func TestBuildFilter_SkipsConditionsWithoutField(t *testing.T) {
	f := BuildFilter(group("and", cond("", "eq", "x"), cond("b", "eq", "2")))
	require.NotNil(t, f)
	condB, ok := f["b"]
	require.True(t, ok)
	require.Equal(t, map[string]any{"$eq": "2"}, condB)
}

func TestBuildFilter_SkipsEmptyValueSingle(t *testing.T) {
	f := BuildFilter(group("and", cond("a", "eq", ""), cond("b", "eq", "2")))
	require.NotNil(t, f)
	_, hasA := f["a"]
	require.False(t, hasA)
	require.Equal(t, map[string]any{"$eq": "2"}, f["b"])
}

func TestBuildFilter_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("a", "eq", "1"), *group("and"))
	f := BuildFilter(root)
	require.NotNil(t, f)
	require.Equal(t, map[string]any{"$eq": "1"}, f["a"])
}

func TestSplitTokens(t *testing.T) {
	require.Equal(t, []string{"a", "b", "c"}, splitTokens("a,b,c"))
	require.Equal(t, []string{"hello", "world"}, splitTokens(" hello , world "))
	require.Empty(t, splitTokens(""))
	require.Empty(t, splitTokens("   "))
}

func TestParseBool(t *testing.T) {
	require.True(t, parseBool("true"))
	require.True(t, parseBool("YES"))
	require.True(t, parseBool("1"))
	require.True(t, parseBool("checked"))
	require.False(t, parseBool("false"))
	require.False(t, parseBool(""))
}
