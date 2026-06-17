package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, category, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Category: category, Op: op, Value: value}
}

func group(connector string, children ...FilterNode) FilterNode {
	return FilterNode{Kind: "group", Connector: connector, Children: children}
}

// jsonEq marshals the built filter and compares it to the expected JSON.
func jsonEq(t *testing.T, expected string, got map[string]any) {
	t.Helper()
	b, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(b))
}

func TestBuildFilter(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		require.Nil(t, BuildFilter(nil))
		require.Nil(t, BuildFilter(&FilterNode{Kind: "group"}))
	})

	t.Run("single text condition", func(t *testing.T) {
		g := group("and", cond("Name", "text", "equals", "Alice"))
		jsonEq(t, `{"property":"Name","rich_text":{"equals":"Alice"}}`, BuildFilter(&g))
	})

	t.Run("number value is coerced to a JSON number", func(t *testing.T) {
		g := group("and", cond("MRR", "number", "greater_than", "49.5"))
		jsonEq(t, `{"property":"MRR","number":{"greater_than":49.5}}`, BuildFilter(&g))
	})

	t.Run("checkbox value is coerced to a JSON bool", func(t *testing.T) {
		g := group("and", cond("Active", "checkbox", "equals", "true"))
		jsonEq(t, `{"property":"Active","checkbox":{"equals":true}}`, BuildFilter(&g))
	})

	t.Run("unary operator encodes true", func(t *testing.T) {
		g := group("and", cond("Notes", "text", "is_empty", ""))
		jsonEq(t, `{"property":"Notes","rich_text":{"is_empty":true}}`, BuildFilter(&g))
	})

	t.Run("group connector with two conditions", func(t *testing.T) {
		g := group("or",
			cond("Age", "number", "greater_than", "30"),
			cond("Age", "number", "less_than", "10"),
		)
		jsonEq(t, `{"or":[
			{"property":"Age","number":{"greater_than":30}},
			{"property":"Age","number":{"less_than":10}}
		]}`, BuildFilter(&g))
	})

	t.Run("list 'in' expands into an or-group of equals", func(t *testing.T) {
		g := group("and", cond("Status", "select", "in", "open, closed"))
		jsonEq(t, `{"or":[
			{"property":"Status","select":{"equals":"open"}},
			{"property":"Status","select":{"equals":"closed"}}
		]}`, BuildFilter(&g))
	})

	t.Run("list 'not_in' expands into an and-group of does_not_equal", func(t *testing.T) {
		g := group("and", cond("Status", "select", "not_in", "open,closed"))
		jsonEq(t, `{"and":[
			{"property":"Status","select":{"does_not_equal":"open"}},
			{"property":"Status","select":{"does_not_equal":"closed"}}
		]}`, BuildFilter(&g))
	})

	t.Run("list with a single token collapses to a plain condition", func(t *testing.T) {
		g := group("and", cond("Status", "select", "in", "open"))
		jsonEq(t, `{"property":"Status","select":{"equals":"open"}}`, BuildFilter(&g))
	})

	t.Run("nested groups", func(t *testing.T) {
		g := group("and",
			cond("Status", "select", "equals", "open"),
			group("or", cond("Age", "number", "greater_than", "30"), cond("Age", "number", "less_than", "10")),
		)
		jsonEq(t, `{"and":[
			{"property":"Status","select":{"equals":"open"}},
			{"or":[
				{"property":"Age","number":{"greater_than":30}},
				{"property":"Age","number":{"less_than":10}}
			]}
		]}`, BuildFilter(&g))
	})

	t.Run("drops incomplete conditions", func(t *testing.T) {
		g := group("and",
			cond("", "text", "equals", "x"),
			cond("Age", "number", "greater_than", ""),
			cond("Name", "text", "equals", "Bob"),
		)
		jsonEq(t, `{"property":"Name","rich_text":{"equals":"Bob"}}`, BuildFilter(&g))
	})

	t.Run("defaults op per category", func(t *testing.T) {
		g := group("and", cond("Name", "text", "", "Bob"))
		jsonEq(t, `{"property":"Name","rich_text":{"equals":"Bob"}}`, BuildFilter(&g))
	})

	t.Run("multi_select contains", func(t *testing.T) {
		g := group("and", cond("Tags", "multi_select", "contains", "vip"))
		jsonEq(t, `{"property":"Tags","multi_select":{"contains":"vip"}}`, BuildFilter(&g))
	})
}

func TestLoadQuery_ParsesFilterTree(t *testing.T) {
	raw := []byte(`{
		"queryType":"records",
		"databaseId":"db1",
		"filterTree":"{\"kind\":\"group\",\"connector\":\"and\",\"children\":[{\"kind\":\"condition\",\"field\":\"Name\",\"category\":\"text\",\"op\":\"equals\",\"value\":\"Alice\"}]}"
	}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.NotNil(t, q.filter)
	jsonEq(t, `{"property":"Name","rich_text":{"equals":"Alice"}}`, BuildFilter(q.filter))
}

func TestLoadQuery_NoFilterTree(t *testing.T) {
	raw := []byte(`{"queryType":"records","databaseId":"db1"}`)
	q, err := LoadQuery(raw)
	require.NoError(t, err)
	require.Nil(t, q.filter)
}

func TestOperatorArity(t *testing.T) {
	require.Equal(t, "none", operatorArity("is_empty"))
	require.Equal(t, "list", operatorArity("in"))
	require.Equal(t, "single", operatorArity("equals"))
}

func TestCategoryForNotionType(t *testing.T) {
	require.Equal(t, "number", categoryForNotionType("number"))
	require.Equal(t, "text", categoryForNotionType("title"))
	require.Equal(t, "text", categoryForNotionType("rich_text"))
	require.Equal(t, "checkbox", categoryForNotionType("checkbox"))
	require.Equal(t, "date", categoryForNotionType("created_time"))
	require.Equal(t, "multi_select", categoryForNotionType("multi_select"))
	require.Equal(t, "people", categoryForNotionType("last_edited_by"))
	require.Equal(t, "text", categoryForNotionType("relation"))
}
