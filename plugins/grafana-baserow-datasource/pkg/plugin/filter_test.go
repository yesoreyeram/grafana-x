package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Op: op, Value: value}
}

func group(connector string, children ...FilterNode) FilterNode {
	return FilterNode{Kind: "group", Connector: connector, Children: children}
}

// parseTree unmarshals the JSON produced by BuildFilters for assertions.
func parseTree(t *testing.T, s string) baserowFilterTree {
	t.Helper()
	var tree baserowFilterTree
	require.NoError(t, json.Unmarshal([]byte(s), &tree))
	return tree
}

func TestBuildFilters(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		require.Equal(t, "", BuildFilters(nil))
		require.Equal(t, "", BuildFilters(&FilterNode{Kind: "group"}))
	})

	t.Run("single condition", func(t *testing.T) {
		g := group("and", cond("Name", "equal", "Alice"))
		tree := parseTree(t, BuildFilters(&g))
		require.Equal(t, "AND", tree.FilterType)
		require.Len(t, tree.Filters, 1)
		require.Equal(t, baserowFilter{Field: "Name", Type: "equal", Value: "Alice"}, tree.Filters[0])
		require.Empty(t, tree.Groups)
	})

	t.Run("group connector OR", func(t *testing.T) {
		g := group("or", cond("Age", "higher_than", "30"), cond("Age", "lower_than", "10"))
		tree := parseTree(t, BuildFilters(&g))
		require.Equal(t, "OR", tree.FilterType)
		require.Len(t, tree.Filters, 2)
	})

	t.Run("unary operator drops value", func(t *testing.T) {
		g := group("and", cond("Notes", "empty", "ignored"))
		tree := parseTree(t, BuildFilters(&g))
		require.Equal(t, baserowFilter{Field: "Notes", Type: "empty", Value: ""}, tree.Filters[0])
	})

	t.Run("nested groups", func(t *testing.T) {
		g := group("and",
			cond("Status", "equal", "open"),
			group("or", cond("Age", "higher_than", "30"), cond("Age", "lower_than", "10")),
		)
		tree := parseTree(t, BuildFilters(&g))
		require.Equal(t, "AND", tree.FilterType)
		require.Len(t, tree.Filters, 1)
		require.Len(t, tree.Groups, 1)
		require.Equal(t, "OR", tree.Groups[0].FilterType)
		require.Len(t, tree.Groups[0].Filters, 2)
	})

	t.Run("drops incomplete conditions", func(t *testing.T) {
		g := group("and",
			cond("", "equal", "x"),
			cond("Name", "equal", "Bob"),
		)
		tree := parseTree(t, BuildFilters(&g))
		require.Len(t, tree.Filters, 1)
		require.Equal(t, "Name", tree.Filters[0].Field)
	})

	t.Run("defaults op to equal", func(t *testing.T) {
		g := group("and", cond("Name", "", "Bob"))
		tree := parseTree(t, BuildFilters(&g))
		require.Equal(t, "equal", tree.Filters[0].Type)
	})

	t.Run("empty nested group is omitted", func(t *testing.T) {
		g := group("and",
			cond("Name", "equal", "Bob"),
			group("or"), // no children -> dropped
		)
		tree := parseTree(t, BuildFilters(&g))
		require.Len(t, tree.Filters, 1)
		require.Empty(t, tree.Groups)
	})
}

func TestLoadQuery_ParsesFilterTree(t *testing.T) {
	raw := []byte(`{
		"queryType":"records",
		"tableId":"1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"Name\",\"op\":\"equal\",\"value\":\"Alice\"}]}"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	tree := parseTree(t, BuildFilters(q.filter))
	require.Equal(t, "Name", tree.Filters[0].Field)
}

func TestLoadQuery_DefaultsQueryType(t *testing.T) {
	raw := []byte(`{"tableId":"1"}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.Equal(t, "records", q.QueryType)
	require.Nil(t, q.filter)
}

func TestOperatorArity(t *testing.T) {
	require.Equal(t, "none", operatorArity("empty"))
	require.Equal(t, "none", operatorArity("not_empty"))
	require.Equal(t, "single", operatorArity("equal"))
}
