package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildJSON compiles a filter tree and marshals the result to JSON for assertion.
func buildJSON(t *testing.T, root *FilterNode) string {
	t.Helper()
	f := BuildFilter(root)
	if f == nil {
		return ""
	}
	b, err := json.Marshal(f)
	require.NoError(t, err)
	return string(b)
}

func cond(field, category, op, value string) FilterNode {
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

func TestBuildFilter_SingleConditionEnvelope(t *testing.T) {
	got := buildJSON(t, group("and", cond("Name", "text", "is", "Alice")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Name","operator":"is","value":"Alice"}]}`, got)
}

func TestBuildFilter_AndOr(t *testing.T) {
	got := buildJSON(t, group("and", cond("A", "text", "is", "1"), cond("B", "text", "is", "2")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[
		{"fieldId":"A","operator":"is","value":"1"},
		{"fieldId":"B","operator":"is","value":"2"}
	]}`, got)

	got = buildJSON(t, group("or", cond("A", "text", "is", "1"), cond("B", "text", "is", "2")))
	require.JSONEq(t, `{"conjunction":"or","filterSet":[
		{"fieldId":"A","operator":"is","value":"1"},
		{"fieldId":"B","operator":"is","value":"2"}
	]}`, got)
}

func TestBuildFilter_NestedGroups(t *testing.T) {
	root := group("and",
		cond("A", "text", "is", "1"),
		*group("or", cond("B", "number", "isGreater", "10"), cond("C", "number", "isLess", "5")),
	)
	got := buildJSON(t, root)
	require.JSONEq(t, `{"conjunction":"and","filterSet":[
		{"fieldId":"A","operator":"is","value":"1"},
		{"conjunction":"or","filterSet":[
			{"fieldId":"B","operator":"isGreater","value":10},
			{"fieldId":"C","operator":"isLess","value":5}
		]}
	]}`, got)
}

func TestBuildFilter_NumberCoercion(t *testing.T) {
	got := buildJSON(t, group("and", cond("Age", "number", "isGreaterEqual", "18")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Age","operator":"isGreaterEqual","value":18}]}`, got)

	// Non-numeric value falls back to a string so the query still runs.
	got = buildJSON(t, group("and", cond("Age", "number", "is", "abc")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Age","operator":"is","value":"abc"}]}`, got)
}

func TestBuildFilter_BooleanCoercion(t *testing.T) {
	got := buildJSON(t, group("and", cond("Done", "boolean", "is", "true")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Done","operator":"is","value":true}]}`, got)

	got = buildJSON(t, group("and", cond("Done", "boolean", "is", "no")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Done","operator":"is","value":false}]}`, got)
}

func TestBuildFilter_DateValueShape(t *testing.T) {
	got := buildJSON(t, group("and", cond("When", "date", "isBefore", "2024-01-15")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[
		{"fieldId":"When","operator":"isBefore","value":{"mode":"exactDate","exactDate":"2024-01-15","timeZone":"UTC"}}
	]}`, got)
}

func TestBuildFilter_UnaryOperatorsNullValue(t *testing.T) {
	got := buildJSON(t, group("and", cond("Notes", "text", "isEmpty", "")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Notes","operator":"isEmpty","value":null}]}`, got)

	got = buildJSON(t, group("and", cond("Notes", "text", "isNotEmpty", "")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Notes","operator":"isNotEmpty","value":null}]}`, got)
}

func TestBuildFilter_ListOperatorsArrayValue(t *testing.T) {
	got := buildJSON(t, group("and", cond("Status", "select", "isAnyOf", "open, closed ,pending")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[
		{"fieldId":"Status","operator":"isAnyOf","value":["open","closed","pending"]}
	]}`, got)

	got = buildJSON(t, group("and", cond("Tags", "multiSelect", "hasAllOf", "a,b")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[
		{"fieldId":"Tags","operator":"hasAllOf","value":["a","b"]}
	]}`, got)
}

func TestBuildFilter_ListOperatorEmptyValueSkipped(t *testing.T) {
	// A list operator with no tokens yields no usable condition -> nil filter.
	require.Nil(t, BuildFilter(group("and", cond("Status", "select", "isAnyOf", " , "))))
}

func TestBuildFilter_SkipsConditionsWithoutField(t *testing.T) {
	got := buildJSON(t, group("and", cond("", "text", "is", "x"), cond("B", "text", "is", "2")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"B","operator":"is","value":"2"}]}`, got)
}

func TestBuildFilter_SkipsSingleConditionWithEmptyValue(t *testing.T) {
	require.Nil(t, BuildFilter(group("and", cond("A", "text", "is", ""))))
}

func TestBuildFilter_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("A", "text", "is", "1"), *group("and"))
	got := buildJSON(t, root)
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"A","operator":"is","value":"1"}]}`, got)
}

func TestBuildFilter_DefaultOperator(t *testing.T) {
	// Empty op defaults to "is" (or "hasAnyOf" for multiSelect).
	got := buildJSON(t, group("and", cond("A", "text", "", "x")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"A","operator":"is","value":"x"}]}`, got)

	got = buildJSON(t, group("and", cond("Tags", "multiSelect", "", "a,b")))
	require.JSONEq(t, `{"conjunction":"and","filterSet":[{"fieldId":"Tags","operator":"hasAnyOf","value":["a","b"]}]}`, got)
}

func TestOperatorArity(t *testing.T) {
	require.Equal(t, arityNone, operatorArity("isEmpty"))
	require.Equal(t, arityNone, operatorArity("isNotEmpty"))
	require.Equal(t, arityList, operatorArity("isAnyOf"))
	require.Equal(t, arityList, operatorArity("hasAllOf"))
	require.Equal(t, aritySingle, operatorArity("is"))
	require.Equal(t, aritySingle, operatorArity("contains"))
	require.Equal(t, aritySingle, operatorArity("isBefore"))
}

func TestSortItemJSON(t *testing.T) {
	items := []SortItem{
		{Field: "fld-1", Direction: "desc"},
		{Field: "fld-2", Direction: "asc"},
	}
	b, err := json.Marshal(items)
	require.NoError(t, err)

	var parsed []SortItem
	require.NoError(t, json.Unmarshal(b, &parsed))
	require.Equal(t, items, parsed)
}
