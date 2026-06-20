package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypesAndPreservesColumns(t *testing.T) {
	records := []map[string]any{
		{"Id": float64(1), "Title": "first", "Active": true, "CreatedAt": "2024-01-02T03:04:05Z"},
		{"Id": float64(2), "Title": "second", "Active": false, "CreatedAt": "2024-02-03T04:05:06Z"},
	}

	frame := recordsToFrame("A", records)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 2, rowLen)
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"Name": "a", "CreatedAt": "2024-02-03T04:05:06Z", "Age": float64(1)},
		{"Name": "b", "CreatedAt": "2024-01-02T03:04:05Z", "Age": float64(2)},
	}
	frame := recordsToFrame("A", records)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	records := []map[string]any{
		{"Name": "b", "CreatedAt": "2024-02-03T04:05:06Z"},
		{"Name": "a", "CreatedAt": "2024-01-02T03:04:05Z"},
		{"Name": "c", "CreatedAt": "2024-03-04T05:06:07Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "CreatedAt", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"Name": "c", "CreatedAt": "2024-03-04T05:06:07Z"},
		{"Name": "b", "CreatedAt": "2024-02-03T04:05:06Z"},
		{"Name": "a", "CreatedAt": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "CreatedAt", frame.Fields[0].Name)
	require.Equal(t, "Name", frame.Fields[1].Name)

	require.Equal(t, "c", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "b", *frame.Fields[1].At(1).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(2).(*string))
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("B", nil)
	require.Equal(t, "B", frame.RefID)
	require.Equal(t, 0, len(frame.Fields))
}

func TestRecordsToFrame_MixedTypesFallBackToString(t *testing.T) {
	records := []map[string]any{
		{"val": float64(1)},
		{"val": "text"},
	}
	frame := recordsToFrame("C", records)
	require.Equal(t, 1, len(frame.Fields))
	_, ok := frame.Fields[0].At(0).(*string)
	require.True(t, ok)
}

func TestInferColumnType(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("b", []map[string]any{{"b": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime_MondayDateFormats(t *testing.T) {
	cases := []string{
		"2024-01-01T00:00:00Z",     // RFC3339 (created_at/updated_at)
		"2024-01-01T00:00:00.000Z", // with millis
		"2024-01-01 00:00:00 UTC",  // monday's space-separated UTC format
		"2024-01-01",               // date-only
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}

	_, ok := toTime("just some text")
	require.False(t, ok)
}

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Equal(t, "A", frame.RefID)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, 1, frame.Fields[0].Len())

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeNumericWide, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
	require.Equal(t, data.FieldTypeInt64, frame.Fields[0].Type())
}

// ---------------------------------------------------------------------------
// Node + item flattening
// ---------------------------------------------------------------------------

func TestFlattenNode_RelationTypes(t *testing.T) {
	row := flattenNode(json.RawMessage(`{
		"id": "9",
		"name": "Tasks",
		"items_count": 5,
		"workspace": {"id": "w1", "name": "Main"},
		"owners": [{"id":"1","name":"Alice"},{"id":"2","name":"Bob"}]
	}`))

	require.Equal(t, "9", row["id"])
	require.Equal(t, "Tasks", row["name"])
	require.Equal(t, float64(5), row["items_count"])
	require.Equal(t, "Main", row["workspace"])    // nested object -> name
	require.Equal(t, "Alice, Bob", row["owners"]) // array of named objects -> joined
}

func TestFlattenItem_WithColumns(t *testing.T) {
	row := flattenItem(json.RawMessage(`{
		"id": "11",
		"name": "Task A",
		"state": "active",
		"created_at": "2024-01-02T03:04:05Z",
		"group": {"id":"g1","title":"Doing"},
		"board": {"id":"1","name":"Tasks"},
		"column_values": [
			{"id":"status","column":{"title":"Status"},"text":"Working"},
			{"id":"person","column":{"title":"Owner"},"text":"Alice"},
			{"id":"empty","column":{"title":"Notes"},"text":""}
		]
	}`), true, false)

	require.Equal(t, "11", row["id"])
	require.Equal(t, "Task A", row["name"])
	require.Equal(t, "Doing", row["group"]) // nested -> title
	require.Equal(t, "Tasks", row["board"]) // nested -> name
	require.Equal(t, "Working", row["Status"])
	require.Equal(t, "Alice", row["Owner"])
	require.Nil(t, row["Notes"]) // empty text -> nil
	require.NotContains(t, row, "column_values")
}

func TestFlattenItem_WithoutColumns(t *testing.T) {
	row := flattenItem(json.RawMessage(`{
		"id": "11",
		"name": "Task A",
		"column_values": [{"id":"status","column":{"title":"Status"},"text":"Working"}]
	}`), false, false)
	require.Equal(t, "Task A", row["name"])
	require.NotContains(t, row, "Status")
	require.NotContains(t, row, "column_values")
}

func TestFlattenItem_ColumnTitleFallsBackToID(t *testing.T) {
	row := flattenItem(json.RawMessage(`{
		"id": "11",
		"column_values": [{"id":"text_1","column":{"title":""},"text":"hi"}]
	}`), true, false)
	require.Equal(t, "hi", row["text_1"]) // falls back to column id
}

func TestFlattenItem_ColumnNameCollision(t *testing.T) {
	row := flattenItem(json.RawMessage(`{
		"id": "11",
		"name": "Task A",
		"column_values": [{"id":"x","column":{"title":"name"},"text":"override?"}]
	}`), true, false)
	require.Equal(t, "Task A", row["name"])             // core field preserved
	require.Equal(t, "override?", row["name (column)"]) // column renamed to avoid clobber
}

func TestFlattenItem_CheckboxBecomesBool(t *testing.T) {
	row := flattenItem(json.RawMessage(`{
		"id": "11",
		"column_values": [
			{"id":"done","type":"checkbox","column":{"title":"Done"},"text":"v"},
			{"id":"todo","type":"checkbox","column":{"title":"Pending"},"text":""}
		]
	}`), true, false)
	require.Equal(t, true, row["Done"])     // "v" -> checked
	require.Equal(t, false, row["Pending"]) // "" -> unchecked
}

func TestFlattenItem_HideSystemColumns(t *testing.T) {
	row := flattenItem(json.RawMessage(`{
		"id": "11",
		"name": "Task A",
		"column_values": [
			{"id":"status","type":"status","column":{"title":"Status"},"text":"Working"},
			{"id":"subitems","type":"subtasks","column":{"title":"Subitems"},"text":"1 item"},
			{"id":"updated","type":"last_updated","column":{"title":"Last updated"},"text":"yesterday"}
		]
	}`), true, true)
	require.Equal(t, "Working", row["Status"]) // user column kept
	require.NotContains(t, row, "Subitems")    // system column hidden
	require.NotContains(t, row, "Last updated")
}

func TestFlattenItem_FlowsThroughFrame(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"id":"11","name":"A","created_at":"2024-01-02T03:04:05Z","column_values":[{"id":"num","column":{"title":"Num"},"text":"1"}]}`),
		json.RawMessage(`{"id":"12","name":"B","created_at":"2024-01-03T03:04:05Z","column_values":[{"id":"num","column":{"title":"Num"},"text":"2"}]}`),
	}
	records := make([]map[string]any, 0, len(items))
	for _, it := range items {
		records = append(records, flattenItem(it, true, false))
	}
	frame := recordsToFrame("A", records)

	num, _ := frame.FieldByName("Num")
	require.NotNil(t, num)
	// monday column `text` is always a string, so it stays a string field.
	require.Equal(t, data.FieldTypeNullableString, num.Type())
	require.Equal(t, "1", *num.At(0).(*string))

	created, _ := frame.FieldByName("created_at")
	require.NotNil(t, created)
	require.Equal(t, data.FieldTypeNullableTime, created.Type())
}
