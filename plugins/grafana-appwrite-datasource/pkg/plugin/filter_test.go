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

func TestBuildFilterQueries_Nil(t *testing.T) {
	require.Nil(t, BuildFilterQueries(nil))
}

func TestBuildFilterQueries_Empty(t *testing.T) {
	require.Nil(t, BuildFilterQueries(group("and")))
}

func TestBuildFilterQueries_SingleCondition(t *testing.T) {
	got := BuildFilterQueries(group("and", cond("status", "equal", "active")))
	require.Equal(t, []string{`{"method":"equal","attribute":"status","values":["active"]}`}, got)
}

func TestBuildFilterQueries_Operators(t *testing.T) {
	cases := []struct {
		op    string
		value string
		want  string
	}{
		{"equal", "x", `{"method":"equal","attribute":"F","values":["x"]}`},
		{"notEqual", "x", `{"method":"notEqual","attribute":"F","values":["x"]}`},
		{"greaterThan", "5", `{"method":"greaterThan","attribute":"F","values":[5]}`},
		{"greaterThanEqual", "5", `{"method":"greaterThanEqual","attribute":"F","values":[5]}`},
		{"lessThan", "5", `{"method":"lessThan","attribute":"F","values":[5]}`},
		{"lessThanEqual", "5", `{"method":"lessThanEqual","attribute":"F","values":[5]}`},
		{"equal", "5.5", `{"method":"equal","attribute":"F","values":[5.5]}`},
		{"equal", "true", `{"method":"equal","attribute":"F","values":[true]}`},
		{"equal", "false", `{"method":"equal","attribute":"F","values":[false]}`},
		{"contains", "ab", `{"method":"contains","attribute":"F","values":["ab"]}`},
		{"notContains", "ab", `{"method":"notContains","attribute":"F","values":["ab"]}`},
		{"search", "ab", `{"method":"search","attribute":"F","values":["ab"]}`},
		{"startsWith", "ab", `{"method":"startsWith","attribute":"F","values":["ab"]}`},
		{"endsWith", "ab", `{"method":"endsWith","attribute":"F","values":["ab"]}`},
		{"isNull", "", `{"method":"isNull","attribute":"F"}`},
		{"isNotNull", "", `{"method":"isNotNull","attribute":"F"}`},
		{"unknown_op", "x", `{"method":"equal","attribute":"F","values":["x"]}`}, // falls back
		// String operators must keep numeric-looking values as strings.
		{"contains", "5", `{"method":"contains","attribute":"F","values":["5"]}`},
	}
	for _, tc := range cases {
		got := BuildFilterQueries(group("and", cond("F", tc.op, tc.value)))
		require.Equal(t, []string{tc.want}, got, "op=%s", tc.op)
	}
}

func TestBuildFilterQueries_AndSplitsIntoSeparateQueries(t *testing.T) {
	got := BuildFilterQueries(group("and", cond("A", "equal", "1"), cond("B", "equal", "2")))
	require.Equal(t, []string{
		`{"method":"equal","attribute":"A","values":[1]}`,
		`{"method":"equal","attribute":"B","values":[2]}`,
	}, got)
}

func TestBuildFilterQueries_OrWrapsInSingleQuery(t *testing.T) {
	got := BuildFilterQueries(group("or", cond("A", "equal", "1"), cond("B", "equal", "2")))
	require.Len(t, got, 1)
	require.Equal(t,
		`{"method":"or","values":["{\"method\":\"equal\",\"attribute\":\"A\",\"values\":[1]}","{\"method\":\"equal\",\"attribute\":\"B\",\"values\":[2]}"]}`,
		got[0])
}

func TestBuildFilterQueries_NestedGroup(t *testing.T) {
	root := group("and",
		cond("A", "equal", "1"),
		*group("or", cond("B", "equal", "2"), cond("C", "equal", "3")),
	)
	got := BuildFilterQueries(root)
	require.Len(t, got, 2)
	require.Equal(t, `{"method":"equal","attribute":"A","values":[1]}`, got[0])
	require.Contains(t, got[1], `"method":"or"`)
}

func TestBuildFilterQueries_SkipsConditionsWithoutAttribute(t *testing.T) {
	got := BuildFilterQueries(group("and", cond("", "equal", "x"), cond("B", "equal", "2")))
	require.Equal(t, []string{`{"method":"equal","attribute":"B","values":[2]}`}, got)
}

func TestBuildFilterQueries_SkipsEmptyNestedGroup(t *testing.T) {
	root := group("and", cond("A", "equal", "1"), *group("and"))
	got := BuildFilterQueries(root)
	require.Equal(t, []string{`{"method":"equal","attribute":"A","values":[1]}`}, got)
}

func TestOrderQueries(t *testing.T) {
	got := orderQueries([]SortItem{
		{Attribute: "age", Direction: "desc"},
		{Attribute: "name", Direction: "asc"},
		{Attribute: "", Direction: "asc"}, // skipped
	})
	require.Equal(t, []string{
		`{"method":"orderDesc","attribute":"age"}`,
		`{"method":"orderAsc","attribute":"name"}`,
	}, got)
}

func TestSelectQuery(t *testing.T) {
	// Without hiding system fields, the identity attributes are appended.
	q, ok := selectQuery("name, age", false)
	require.True(t, ok)
	require.Equal(t, `{"method":"select","values":["name","age","$id","$createdAt","$updatedAt"]}`, q)

	// When hiding system fields, only the requested attributes are selected.
	q, ok = selectQuery("name, age", true)
	require.True(t, ok)
	require.Equal(t, `{"method":"select","values":["name","age"]}`, q)

	_, ok = selectQuery("", false)
	require.False(t, ok)
	_, ok = selectQuery("  ,  ", false)
	require.False(t, ok)
}

func TestCoerce(t *testing.T) {
	require.Equal(t, true, coerce("true"))
	require.Equal(t, false, coerce("false"))
	require.Equal(t, int64(5), coerce("5"))
	require.Equal(t, int64(-5), coerce("-5"))
	require.Equal(t, 5.5, coerce("5.5"))
	require.Equal(t, "hello", coerce("hello"))
	require.Equal(t, "5a", coerce("5a"))
}
