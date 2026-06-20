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
	// The frame layer must NOT re-sort rows: the order returned by Linear (which
	// honours the query's ordering) must be preserved exactly.
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

func TestRecordsToFrame_OffsetDateTimeNormalisedToUTC(t *testing.T) {
	records := []map[string]any{
		{"ts": "2024-01-01T12:00:00.000+05:30"},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC) // 12:00 +05:30 == 06:30 UTC
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
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

func TestToTime_LinearDateFormats(t *testing.T) {
	cases := []string{
		"2024-01-01T00:00:00.000Z",      // Linear timestamp with millis
		"2024-01-01T12:00:00.000+05:30", // offset zone with millis
		"2024-01-01T00:00:00Z",          // RFC3339
		"2024-01-01",                    // date only (dueDate / targetDate)
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

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 1, rowLen)
}

// ---------------------------------------------------------------------------
// Node flattening
// ---------------------------------------------------------------------------

func nodeFromJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	return flattenNode(json.RawMessage(raw))
}

func TestFlattenNode_AllRelationTypes(t *testing.T) {
	row := nodeFromJSON(t, `{
		"id": "abc",
		"identifier": "ENG-123",
		"title": "Fix login",
		"priority": 2,
		"estimate": 3.5,
		"completedAt": null,
		"createdAt": "2024-01-02T03:04:05.000Z",
		"state": {"name": "In Progress", "type": "started"},
		"assignee": {"name": "Alice", "email": "a@b.com"},
		"team": {"key": "ENG", "name": "Engineering"},
		"cycle": {"number": 7, "name": ""},
		"labels": {"nodes": [{"name": "bug"}, {"name": "p1"}]}
	}`)

	require.Equal(t, "abc", row["id"])
	require.Equal(t, "ENG-123", row["identifier"])
	require.Equal(t, "Fix login", row["title"])
	require.Equal(t, float64(2), row["priority"])
	require.Equal(t, 3.5, row["estimate"])
	require.Nil(t, row["completedAt"])
	require.Equal(t, "2024-01-02T03:04:05.000Z", row["createdAt"])
	require.Equal(t, "In Progress", row["state"]) // nested object -> name
	require.Equal(t, "Alice", row["assignee"])    // nested object -> name
	require.Equal(t, "Engineering", row["team"])  // nested object -> name (preferred over key)
	require.EqualValues(t, 7, row["cycle"])       // empty name falls back to number
	require.Equal(t, "bug, p1", row["labels"])    // connection -> joined names
}

func TestFlattenNode_TeamPrefersName(t *testing.T) {
	row := nodeFromJSON(t, `{"team": {"key": "ENG", "name": "Engineering"}}`)
	// name is preferred over key.
	require.Equal(t, "Engineering", row["team"])
}

func TestFlattenNode_EmptyConnectionIsNil(t *testing.T) {
	row := nodeFromJSON(t, `{"labels": {"nodes": []}}`)
	require.Nil(t, row["labels"])
}

func TestFlattenNode_FlowsThroughFrame(t *testing.T) {
	nodes := []json.RawMessage{
		json.RawMessage(`{"identifier":"ENG-1","estimate":1.0,"state":{"name":"Todo"}}`),
		json.RawMessage(`{"identifier":"ENG-2","estimate":2.0,"state":{"name":"Done"}}`),
	}
	records := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		records = append(records, flattenNode(n))
	}
	frame := recordsToFrame("A", records)

	est, _ := frame.FieldByName("estimate")
	require.NotNil(t, est)
	require.Equal(t, data.FieldTypeNullableFloat64, est.Type())
	require.EqualValues(t, 1.0, *est.At(0).(*float64))

	stateField, _ := frame.FieldByName("state")
	require.NotNil(t, stateField)
	require.Equal(t, "Todo", *stateField.At(0).(*string))
}
