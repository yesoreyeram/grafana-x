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
	params := BuildFilter(nil)
	require.Len(t, params, 0)
}

func TestBuildFilter_EmptyGroup(t *testing.T) {
	params := BuildFilter(group("and"))
	require.Len(t, params, 0)
}

func TestBuildFilter_SingleCondition(t *testing.T) {
	params := BuildFilter(group("and", cond("status", "eq", "published")))
	require.Len(t, params, 1)
	require.Equal(t, "published", params.Get("filters[status][$eq]"))
}

func TestBuildFilter_Operators(t *testing.T) {
	tests := []struct {
		op    string
		value string
		key   string
		want  string
	}{
		{"eq", "x", "filters[F][$eq]", "x"},
		{"ne", "x", "filters[F][$ne]", "x"},
		{"gt", "5", "filters[F][$gt]", "5"},
		{"gte", "5", "filters[F][$gte]", "5"},
		{"lt", "5", "filters[F][$lt]", "5"},
		{"lte", "5", "filters[F][$lte]", "5"},
		{"contains", "ab", "filters[F][$contains]", "ab"},
		{"containsi", "ab", "filters[F][$containsi]", "ab"},
		{"notContains", "ab", "filters[F][$notContains]", "ab"},
		{"startsWith", "ab", "filters[F][$startsWith]", "ab"},
		{"endsWith", "ab", "filters[F][$endsWith]", "ab"},
		{"null", "", "filters[F][$null]", "true"},
		{"notNull", "", "filters[F][$notNull]", "true"},
	}
	for _, tt := range tests {
		params := BuildFilter(group("and", cond("F", tt.op, tt.value)))
		require.Equal(t, tt.want, params.Get(tt.key), "op=%s", tt.op)
	}
}

func TestBuildFilter_InUsesIndexedArray(t *testing.T) {
	params := BuildFilter(group("and", cond("tag", "in", "a,b,c")))
	require.Equal(t, "a", params.Get("filters[tag][$in][0]"))
	require.Equal(t, "b", params.Get("filters[tag][$in][1]"))
	require.Equal(t, "c", params.Get("filters[tag][$in][2]"))
	require.Equal(t, "", params.Get("filters[tag][$in]"))
}

func TestBuildFilter_NotInUsesIndexedArray(t *testing.T) {
	params := BuildFilter(group("and", cond("tag", "notIn", "x,y")))
	require.Equal(t, "x", params.Get("filters[tag][$notIn][0]"))
	require.Equal(t, "y", params.Get("filters[tag][$notIn][1]"))
}

func TestBuildFilter_UnknownOperatorFallsBackToEq(t *testing.T) {
	params := BuildFilter(group("and", cond("F", "bogus", "v")))
	require.Equal(t, "v", params.Get("filters[F][$eq]"))
}

func TestBuildFilter_And(t *testing.T) {
	params := BuildFilter(group("and", cond("a", "eq", "1"), cond("b", "eq", "2")))
	require.Len(t, params, 2)
	require.Equal(t, "1", params.Get("filters[$and][0][a][$eq]"))
	require.Equal(t, "2", params.Get("filters[$and][1][b][$eq]"))
}

func TestBuildFilter_Or(t *testing.T) {
	params := BuildFilter(group("or", cond("a", "eq", "1"), cond("b", "eq", "2")))
	require.Len(t, params, 2)
	require.Equal(t, "1", params.Get("filters[$or][0][a][$eq]"))
	require.Equal(t, "2", params.Get("filters[$or][1][b][$eq]"))
}

func TestBuildFilter_NestedGroups(t *testing.T) {
	root := group("and",
		cond("a", "eq", "1"),
		*group("or", cond("b", "eq", "2"), cond("c", "eq", "3")),
	)
	params := BuildFilter(root)
	require.Len(t, params, 3)
	require.Equal(t, "1", params.Get("filters[$and][0][a][$eq]"))
	require.Equal(t, "2", params.Get("filters[$and][1][$or][0][b][$eq]"))
	require.Equal(t, "3", params.Get("filters[$and][1][$or][1][c][$eq]"))
}

func TestBuildFilter_SkipsConditionsWithoutField(t *testing.T) {
	params := BuildFilter(group("and", cond("", "eq", "x"), cond("b", "eq", "2")))
	require.Len(t, params, 1)
	require.Equal(t, "2", params.Get("filters[b][$eq]"))
}

func TestBuildFilter_SkipsEmptyNestedGroup(t *testing.T) {
	// An empty nested group must not leave a gap in the $and index sequence.
	root := group("and", cond("a", "eq", "1"), *group("and"))
	params := BuildFilter(root)
	require.Len(t, params, 1)
	require.Equal(t, "1", params.Get("filters[a][$eq]"))
}

func TestParseListValue(t *testing.T) {
	require.Equal(t, []string{"a", "b", "c"}, parseListValue("a,b,c"))
	require.Equal(t, []string{"hello", "world"}, parseListValue(" hello , world "))
	require.Nil(t, parseListValue(""))
	require.Nil(t, parseListValue("   "))
}
