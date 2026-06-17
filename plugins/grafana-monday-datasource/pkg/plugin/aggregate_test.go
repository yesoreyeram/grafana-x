package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidAggregation(t *testing.T) {
	for _, a := range []string{aggCount, aggCountDistinct, aggSum, aggAvg, aggMin, aggMax} {
		require.True(t, validAggregation(a), a)
	}
	require.False(t, validAggregation("median"))
}

func TestMondayAggregateFunction(t *testing.T) {
	fn, needs := mondayAggregateFunction(aggCount)
	require.Equal(t, "COUNT_ITEMS", fn)
	require.False(t, needs)

	fn, needs = mondayAggregateFunction(aggSum)
	require.Equal(t, "SUM", fn)
	require.True(t, needs)

	fn, needs = mondayAggregateFunction(aggCountDistinct)
	require.Equal(t, "COUNT_DISTINCT", fn)
	require.True(t, needs)
}

func TestAggregationColumnName(t *testing.T) {
	require.Equal(t, "count", aggregationColumnName(aggCount, ""))
	require.Equal(t, "count", aggregationColumnName(aggCount, "numbers"))
	require.Equal(t, "sum(numbers)", aggregationColumnName(aggSum, "numbers"))
	require.Equal(t, "count_distinct(owner)", aggregationColumnName(aggCountDistinct, "owner"))
}

func TestBuildAggregateQuery_CountByStatus(t *testing.T) {
	doc, vars, err := buildAggregateQuery("123", "status", aggCount, "", nil)
	require.NoError(t, err)
	require.Contains(t, doc, "aggregate(query:")
	require.Contains(t, doc, `from: { type: TABLE, id: "123" }`)
	require.Contains(t, doc, "COUNT_ITEMS")
	require.Contains(t, doc, `group_by: [{ column_id: "status", limit: 500 }]`)
	// The group column's alias must equal its column_id so monday ties the
	// select column to the group_by clause.
	require.Contains(t, doc, `{ type: COLUMN, column: { column_id: "status" }, as: "status" }`)
	require.Contains(t, doc, `as: "result_value"`)
	require.Empty(t, vars) // no filter rules
}

func TestBuildAggregateQuery_SumWithColumnAndFilter(t *testing.T) {
	rules := []any{map[string]any{"column_id": "name", "operator": "contains_text", "compare_value": []any{"x"}}}
	doc, vars, err := buildAggregateQuery("9", "owner", aggSum, "points", rules)
	require.NoError(t, err)
	require.Contains(t, doc, "SUM")
	require.Contains(t, doc, `params: [{ type: COLUMN, column: { column_id: "points" }, as: "points" }]`)
	require.Contains(t, doc, "query: $itemsQuery")
	require.Contains(t, doc, "($itemsQuery: ItemsQuery)")
	require.Contains(t, vars, "itemsQuery")
}

func TestBuildAggregateQuery_NoGroupBy(t *testing.T) {
	doc, _, err := buildAggregateQuery("9", "", aggCount, "", nil)
	require.NoError(t, err)
	require.NotContains(t, doc, "group_by")
}

func TestBuildAggregateQuery_RequiresColumnForNumeric(t *testing.T) {
	_, _, err := buildAggregateQuery("9", "owner", aggSum, "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires a value column")
}

func TestBuildAggregateQuery_RequiresBoard(t *testing.T) {
	_, _, err := buildAggregateQuery("", "owner", aggCount, "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "board id is required")
}

func TestParseAggregateResults_Grouped(t *testing.T) {
	// The group entry's alias is the column id ("status"), matching monday's
	// requirement that the select column be aliased to its column_id.
	data := json.RawMessage(`{"aggregate":{"results":[
		{"entries":[{"alias":"status","value":{"value":"Done"}},{"alias":"result_value","value":{"result":3}}]},
		{"entries":[{"alias":"status","value":{"value":"Working"}},{"alias":"result_value","value":{"result":7}}]},
		{"entries":[{"alias":"status","value":{"value":null}},{"alias":"result_value","value":{"result":1}}]}
	]}}`)
	rows, err := parseAggregateResults(data, "status", "status", "count")
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// Sorted by result desc: Working(7), Done(3), (empty)(1).
	require.Equal(t, "Working", rows[0]["status"])
	require.EqualValues(t, 7, rows[0]["count"])
	require.Equal(t, "Done", rows[1]["status"])
	require.Equal(t, emptyGroupLabel, rows[2]["status"])
}

func TestParseAggregateResults_GroupValueUnderResult(t *testing.T) {
	// Some monday versions return the group value under `result` instead of
	// `value`; we fall back to it.
	data := json.RawMessage(`{"aggregate":{"results":[
		{"entries":[{"alias":"status","value":{"result":"Done"}},{"alias":"result_value","value":{"result":4}}]}
	]}}`)
	rows, err := parseAggregateResults(data, "status", "status", "count")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Done", rows[0]["status"])
	require.EqualValues(t, 4, rows[0]["count"])
}

func TestEntryScalar_Forms(t *testing.T) {
	// Object with result.
	require.EqualValues(t, 3, entryScalar(json.RawMessage(`{"result":3}`)))
	// Object with value (group-by).
	require.Equal(t, "Done", entryScalar(json.RawMessage(`{"value":"Done"}`)))
	// Object with another scalar field only.
	require.Equal(t, "Alice", entryScalar(json.RawMessage(`{"label":"Alice"}`)))
	// Bare scalar.
	require.Equal(t, "#00c875", entryScalar(json.RawMessage(`"#00c875"`)))
	// Null / empty.
	require.Nil(t, entryScalar(json.RawMessage(`null`)))
	require.Nil(t, entryScalar(json.RawMessage(`{"value":null}`)))
}

func TestParseAggregateResults_TextGroupValues(t *testing.T) {
	// Real-world shape for grouping a text column: distinct text values per set.
	data := json.RawMessage(`{"aggregate":{"results":[
		{"entries":[{"alias":"text_mm4dxy6e","value":{"value":"Backend"}},{"alias":"result_value","value":{"result":4}}]},
		{"entries":[{"alias":"text_mm4dxy6e","value":{"value":"Frontend"}},{"alias":"result_value","value":{"result":2}}]}
	]}}`)
	rows, err := parseAggregateResults(data, "text_mm4dxy6e", "text_mm4dxy6e", "count")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "Backend", rows[0]["text_mm4dxy6e"])
	require.EqualValues(t, 4, rows[0]["count"])
	require.Equal(t, "Frontend", rows[1]["text_mm4dxy6e"])
}

func TestParseAggregateResults_Ungrouped(t *testing.T) {
	data := json.RawMessage(`{"aggregate":{"results":[
		{"entries":[{"alias":"result_value","value":{"result":1368}}]}
	]}}`)
	rows, err := parseAggregateResults(data, "", "", "count")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 1368, rows[0]["count"])
}

func TestClient_ListAggregate_CountByStatus(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		gotQuery = req.Query
		_, _ = io.WriteString(w, `{"data":{"aggregate":{"results":[
			{"entries":[{"alias":"status","value":{"value":"Done"}},{"alias":"result_value","value":{"result":5}}]},
			{"entries":[{"alias":"status","value":{"value":"Working"}},{"alias":"result_value","value":{"result":2}}]}
		]}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:   queryTypeItems,
		BoardIds:    []string{"1"},
		GroupBy:     "status",
		Aggregation: aggCount,
	})
	require.NoError(t, err)
	require.Contains(t, gotQuery, "aggregate(query:")
	require.Len(t, records, 2)
	require.Equal(t, "Done", records[0]["status"])
	require.EqualValues(t, 5, records[0]["count"])
}

func TestClient_ListAggregate_MultiBoardAddsBoardColumn(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"data":{"aggregate":{"results":[
			{"entries":[{"alias":"status","value":{"value":"Done"}},{"alias":"result_value","value":{"result":1}}]}
		]}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:   queryTypeItems,
		BoardIds:    []string{"1", "2"},
		GroupBy:     "status",
		Aggregation: aggCount,
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls) // one aggregate call per board
	require.Len(t, records, 2)
	require.Equal(t, "1", records[0]["board_id"])
	require.Equal(t, "2", records[1]["board_id"])
}

func TestClient_ListAggregate_SumSendsFilterAndColumn(t *testing.T) {
	var req graphqlRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		_, _ = io.WriteString(w, `{"data":{"aggregate":{"results":[
			{"entries":[{"alias":"owner","value":{"value":"Alice"}},{"alias":"result_value","value":{"result":13}}]}
		]}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"})
	records, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:         queryTypeItems,
		BoardIds:          []string{"1"},
		GroupBy:           "owner",
		Aggregation:       aggSum,
		AggregationColumn: "points",
		SearchQuery:       "login",
	})
	require.NoError(t, err)
	require.Contains(t, req.Query, "SUM")
	require.Contains(t, req.Variables, "itemsQuery")
	require.Len(t, records, 1)
	require.EqualValues(t, 13, records[0]["sum(points)"])
}

func TestClient_ListAggregate_RequiresBoard(t *testing.T) {
	c := newTestClient(t, "http://example.invalid", Settings{apiToken: "x"})
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:   queryTypeItems,
		GroupBy:     "status",
		Aggregation: aggCount,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "board is required")
}

func TestIsAggregateUnsupportedError(t *testing.T) {
	require.True(t, isAggregateUnsupportedError(
		fmt.Errorf(`monday.com graphql error: Cannot query field "aggregate" on type "Query".`)))
	require.True(t, isAggregateUnsupportedError(
		fmt.Errorf(`monday.com graphql error: Unknown type "AggregateBasicAggregationResult".`)))
	require.False(t, isAggregateUnsupportedError(nil))
	require.False(t, isAggregateUnsupportedError(fmt.Errorf("some other error")))
	require.False(t, isAggregateUnsupportedError(fmt.Errorf("network timeout")))
}

func TestClient_ListAggregate_UnsupportedVersionGivesActionableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[
			{"message":"Cannot query field \"aggregate\" on type \"Query\"."},
			{"message":"Unknown type \"AggregateBasicAggregationResult\"."}
		]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, Settings{apiToken: "x"}) // no API version -> default
	_, err := c.ListRecords(context.Background(), QueryModel{
		QueryType:   queryTypeItems,
		BoardIds:    []string{"1"},
		GroupBy:     "status",
		Aggregation: aggCount,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "aggregate API")
	require.Contains(t, err.Error(), "API version")
	require.Contains(t, err.Error(), defaultAPIVersion) // names the version used
	require.Contains(t, err.Error(), "Group by")
}

func TestAggregate_FlowsThroughFrame(t *testing.T) {
	data := json.RawMessage(`{"aggregate":{"results":[
		{"entries":[{"alias":"status","value":{"value":"Done"}},{"alias":"result_value","value":{"result":3}}]},
		{"entries":[{"alias":"status","value":{"value":"Working"}},{"alias":"result_value","value":{"result":7}}]}
	]}}`)
	rows, err := parseAggregateResults(data, "status", "status", "count")
	require.NoError(t, err)
	frame := recordsToFrame("A", rows)
	require.Equal(t, 2, frame.Rows())

	count, _ := frame.FieldByName("count")
	require.NotNil(t, count)
	require.EqualValues(t, 7, *count.At(0).(*float64)) // Working first (desc)
}
