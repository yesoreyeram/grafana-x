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
		{"id": float64(1), "name": "first", "archived": true, "created_at": "2024-01-02T03:04:05Z"},
		{"id": float64(2), "name": "second", "archived": false, "created_at": "2024-02-03T04:05:06Z"},
	}

	frame := recordsToFrame("A", records, nil)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 2, rowLen)
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"name": "a", "created_at": "2024-02-03T04:05:06Z", "estimate": float64(1)},
		{"name": "b", "created_at": "2024-01-02T03:04:05Z", "estimate": float64(2)},
	}
	frame := recordsToFrame("A", records, nil)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "created_at": "2024-02-03T04:05:06Z"},
		{"name": "a", "created_at": "2024-01-02T03:04:05Z"},
		{"name": "c", "created_at": "2024-03-04T05:06:07Z"},
	}
	frame := recordsToFrame("A", records, nil)

	require.Equal(t, "created_at", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"name": "c", "created_at": "2024-03-04T05:06:07Z"},
		{"name": "b", "created_at": "2024-02-03T04:05:06Z"},
		{"name": "a", "created_at": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records, nil)

	require.Equal(t, "c", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "b", *frame.Fields[1].At(1).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(2).(*string))
}

func TestRecordsToFrame_FieldSelection(t *testing.T) {
	records := []map[string]any{{"id": float64(1), "name": "x", "story_type": "bug"}}
	frame := recordsToFrame("A", records, []string{"name"})
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "name", frame.Fields[0].Name)
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("B", nil, nil)
	require.Equal(t, "B", frame.RefID)
	require.Equal(t, 0, len(frame.Fields))
}

func TestRecordsToFrame_MixedTypesFallBackToString(t *testing.T) {
	records := []map[string]any{
		{"val": float64(1)},
		{"val": "text"},
	}
	frame := recordsToFrame("C", records, nil)
	require.Equal(t, 1, len(frame.Fields))
	_, ok := frame.Fields[0].At(0).(*string)
	require.True(t, ok)
}

func TestInferColumnType_OnlyKnownDateColumnsAreTime(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("b", []map[string]any{{"b": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("created_at", []map[string]any{{"created_at": "2024-01-01T00:00:00Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))

	// A non-date column whose value looks like a date stays a string.
	require.Equal(t, fieldTypeString, inferColumnType("name", []map[string]any{{"name": "2024-01-01T00:00:00Z"}}))
}

func TestToColumnTime(t *testing.T) {
	tm, ok := toColumnTime("created_at", "2024-01-01T00:00:00Z")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), tm.UTC())

	_, ok = toColumnTime("name", "2024-01-01T00:00:00Z")
	require.False(t, ok)

	_, ok = toColumnTime("deadline", float64(3))
	require.False(t, ok)
}

func TestToTime_ShortcutDateFormats(t *testing.T) {
	cases := []string{
		"2024-01-01T00:00:00.000Z",
		"2024-01-01T12:00:00.000+05:30",
		"2024-01-01T00:00:00Z",
		"2024-01-01",
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

// TestFlattenStory_RealSearchShape verifies a story from the search endpoint
// flattens with scalars typed and arrays/objects preserved as compact JSON.
func TestFlattenStory_RealSearchShape(t *testing.T) {
	row := flattenStory(json.RawMessage(`{
		"id": 1,
		"name": "Fix login",
		"story_type": "bug",
		"estimate": 3,
		"archived": false,
		"workflow_state_id": 100,
		"project_id": 10,
		"epic_id": 5,
		"created_at": "2024-01-15T10:00:00Z",
		"owner_ids": ["uuid-1","uuid-2"],
		"label_ids": [1,2],
		"labels": [{"id":1,"name":"bug"},{"id":2,"name":"auth"}]
	}`))

	require.Equal(t, float64(1), row["id"])
	require.Equal(t, "Fix login", row["name"])
	require.Equal(t, "bug", row["story_type"])
	require.Equal(t, float64(3), row["estimate"])
	require.Equal(t, false, row["archived"])
	require.Equal(t, float64(100), row["workflow_state_id"])
	require.Equal(t, float64(10), row["project_id"])
	require.Equal(t, float64(5), row["epic_id"])
	require.Equal(t, "2024-01-15T10:00:00Z", row["created_at"])
	// Arrays preserved as compact JSON (owner_ids / label_ids / labels).
	require.Equal(t, `["uuid-1","uuid-2"]`, row["owner_ids"])
	require.Equal(t, `[1,2]`, row["label_ids"])
	require.Equal(t, `[{"id":1,"name":"bug"},{"id":2,"name":"auth"}]`, row["labels"])
}

func TestFlattenStory_NullAndEmpty(t *testing.T) {
	row := flattenStory(json.RawMessage(`{
		"id": 2,
		"name": "Test",
		"deadline": null,
		"epic_id": null,
		"owner_ids": [],
		"labels": []
	}`))

	require.Equal(t, float64(2), row["id"])
	require.Equal(t, "Test", row["name"])
	require.Nil(t, row["deadline"])
	require.Nil(t, row["epic_id"])
	require.Nil(t, row["owner_ids"])
	require.Nil(t, row["labels"])
}

func TestFlattenStory_FlowsThroughFrame(t *testing.T) {
	nodes := []json.RawMessage{
		json.RawMessage(`{"id":1,"name":"Story 1","estimate":1.0,"story_type":"feature","created_at":"2024-01-01T00:00:00Z"}`),
		json.RawMessage(`{"id":2,"name":"Story 2","estimate":2.0,"story_type":"bug","created_at":"2024-01-02T00:00:00Z"}`),
	}
	records := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		records = append(records, flattenStory(n))
	}
	frame := recordsToFrame("A", records, nil)

	est, _ := frame.FieldByName("estimate")
	require.NotNil(t, est)
	require.Equal(t, data.FieldTypeNullableFloat64, est.Type())
	require.EqualValues(t, 1.0, *est.At(0).(*float64))

	name, _ := frame.FieldByName("name")
	require.NotNil(t, name)
	require.Equal(t, "Story 1", *name.At(0).(*string))
}

func TestSelectColumns(t *testing.T) {
	cols := []string{"id", "name", "story_type"}
	require.Equal(t, []string{"name", "story_type"}, selectColumns(cols, []string{"story_type", "name", "missing"}))
	require.Equal(t, cols, selectColumns(cols, nil))
	require.Equal(t, cols, selectColumns(cols, []string{"   "}))
}
