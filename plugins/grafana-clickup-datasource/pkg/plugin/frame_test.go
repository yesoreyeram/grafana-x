package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlattenTask_ReducesNestedRelations(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"9hx",
		"name":"Fix login bug",
		"status":{"status":"in progress","type":"custom","color":"#fff"},
		"priority":{"priority":"urgent","id":"1"},
		"creator":{"id":183,"username":"Alex","email":"a@b.com"},
		"assignees":[{"username":"Alice"},{"username":"Bob"}],
		"tags":[{"name":"bug"},{"name":"p1"}],
		"list":{"id":"15","name":"Sprint Backlog","access":true},
		"folder":{"id":"6","name":"Mobile Squad"},
		"space":{"id":"7"},
		"date_created":"1567780450202",
		"due_date":null
	}`)

	row := flattenTask(raw)
	require.Equal(t, "9hx", row["id"])
	require.Equal(t, "Fix login bug", row["name"])
	require.Equal(t, "in progress", row["status"])
	require.Equal(t, "custom", row["status_type"])
	require.Equal(t, "urgent", row["priority"])
	require.Equal(t, "Alex", row["creator"])
	require.Equal(t, "Alice, Bob", row["assignees"])
	require.Equal(t, "bug, p1", row["tags"])
	require.Equal(t, "Sprint Backlog", row["list"])
	require.Equal(t, "Mobile Squad", row["folder"])
	require.Equal(t, "1567780450202", row["date_created"])
	require.Nil(t, row["due_date"])
}

func TestFlattenTask_NullPriority(t *testing.T) {
	row := flattenTask(json.RawMessage(`{"id":"1","priority":null}`))
	require.Nil(t, row["priority"])
}

func TestInferColumnType_ClickUpDates(t *testing.T) {
	records := []map[string]any{
		{"date_created": "1567780450202"},
		{"date_created": "1567780450500"},
	}
	require.Equal(t, fieldTypeTime, inferColumnType("date_created", records))

	// A non-date numeric column stays numeric.
	records = []map[string]any{{"points": float64(3)}, {"points": float64(5)}}
	require.Equal(t, fieldTypeNumber, inferColumnType("points", records))
}

func TestToColumnTime_UnixMillis(t *testing.T) {
	tm, ok := toColumnTime("date_created", "1567780450202")
	require.True(t, ok)
	require.Equal(t, time.UnixMilli(1567780450202).UTC(), tm.UTC())

	// Non-date column with a plain number is not a time.
	_, ok = toColumnTime("points", float64(3))
	require.False(t, ok)
}

func TestRecordsToFrame_TimeColumnsFirstAndRowOrderPreserved(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "date_created": "1567780450500"},
		{"name": "a", "date_created": "1567780450202"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Len(t, frame.Fields, 2)
	// Time column should be first.
	require.Equal(t, "date_created", frame.Fields[0].Name)
	require.Equal(t, "name", frame.Fields[1].Name)
	// Row order preserved (NOT re-sorted): "b" then "a".
	require.Equal(t, "b", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(1).(*string))
}

func TestRecordsToFrame_FieldSelection(t *testing.T) {
	records := []map[string]any{{"id": "1", "name": "x", "url": "http://t/1"}}
	frame := recordsToFrame("A", records, []string{"name"})
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "name", frame.Fields[0].Name)
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("A", nil, nil)
	require.Empty(t, frame.Fields)
}

func TestSelectColumns(t *testing.T) {
	cols := []string{"id", "name", "url"}
	require.Equal(t, []string{"name", "url"}, selectColumns(cols, []string{"url", "name", "missing"}))
	require.Equal(t, cols, selectColumns(cols, nil))
	require.Equal(t, cols, selectColumns(cols, []string{"   "}))
}
