package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Op: op, Value: value}
}

func group(connector string, children ...FilterNode) FilterNode {
	return FilterNode{Kind: "group", Connector: connector, Children: children}
}

func TestBuildWhere(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		require.Equal(t, "", BuildWhere(nil, true))
		require.Equal(t, "", BuildWhere(&FilterNode{Kind: "group"}, true))
	})

	t.Run("single condition (v2 quoted)", func(t *testing.T) {
		g := group("and", cond("Name", "eq", "Alice"))
		require.Equal(t, `@("Name",eq,"Alice")`, BuildWhere(&g, true))
	})

	t.Run("single condition (v3 no @ prefix)", func(t *testing.T) {
		g := group("and", cond("Name", "eq", "Alice"))
		require.Equal(t, `("Name",eq,"Alice")`, BuildWhere(&g, false))
	})

	t.Run("group connector", func(t *testing.T) {
		g := group("or", cond("Age", "gt", "30"), cond("Age", "lt", "10"))
		require.Equal(t, `@(("Age",gt,"30")~or("Age",lt,"10"))`, BuildWhere(&g, true))
	})

	t.Run("unary operator", func(t *testing.T) {
		g := group("and", cond("Notes", "blank", ""))
		require.Equal(t, `@("Notes",blank)`, BuildWhere(&g, true))
	})

	t.Run("list operator unquoted tokens", func(t *testing.T) {
		g := group("and", cond("Status", "in", "open, closed"))
		require.Equal(t, `@("Status",in,open,closed)`, BuildWhere(&g, true))
	})

	t.Run("nested groups", func(t *testing.T) {
		g := group("and",
			cond("Status", "eq", "open"),
			group("or", cond("Age", "gt", "30"), cond("Age", "lt", "10")),
		)
		require.Equal(t, `@(("Status",eq,"open")~and(("Age",gt,"30")~or("Age",lt,"10")))`, BuildWhere(&g, true))
	})

	t.Run("drops incomplete conditions", func(t *testing.T) {
		g := group("and",
			cond("", "eq", "x"),
			cond("Age", "gt", ""),
			cond("Name", "eq", "Bob"),
		)
		require.Equal(t, `@("Name",eq,"Bob")`, BuildWhere(&g, true))
	})

	t.Run("escapes quotes", func(t *testing.T) {
		g := group("and", cond("Product", "eq", `Laptop, 15"`))
		require.Equal(t, `@("Product",eq,"Laptop, 15\"")`, BuildWhere(&g, true))
	})

	t.Run("defaults op to eq", func(t *testing.T) {
		g := group("and", cond("Name", "", "Bob"))
		require.Equal(t, `@("Name",eq,"Bob")`, BuildWhere(&g, true))
	})
}

func TestLoadQuery_ParsesFilterTree(t *testing.T) {
	raw := []byte(`{
		"queryType":"records",
		"tableId":"m1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"Name\",\"op\":\"eq\",\"value\":\"Alice\"}]}"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	require.Equal(t, `@("Name",eq,"Alice")`, BuildWhere(q.filter, true))
}

func TestLoadQuery_RawWhereWhenNoTree(t *testing.T) {
	raw := []byte(`{"queryType":"records","tableId":"m1","where":"(Name,eq,Bob)"}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.Nil(t, q.filter)
	require.Equal(t, "(Name,eq,Bob)", q.Where)
}

func TestOperatorArity(t *testing.T) {
	require.Equal(t, "none", operatorArity("blank"))
	require.Equal(t, "list", operatorArity("anyof"))
	require.Equal(t, "single", operatorArity("eq"))
}
