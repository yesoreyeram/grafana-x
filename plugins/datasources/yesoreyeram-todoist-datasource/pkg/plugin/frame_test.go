package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlattenItem_TaskBasicFields(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"6XGgmFVcrG5RRjVr",
		"content":"Buy groceries",
		"description":"Remember the milk",
		"checked":false,
		"priority":1,
		"child_order":5,
		"added_at":"2024-01-15T10:00:00Z",
		"added_by_uid":"u1",
		"responsible_uid":"u2",
		"assigned_by_uid":"u3",
		"project_id":"p1",
		"section_id":"s1",
		"parent_id":null,
		"note_count":3
	}`)

	row := flattenItem(raw)
	require.Equal(t, "6XGgmFVcrG5RRjVr", row["id"])
	require.Equal(t, "Buy groceries", row["content"])
	require.Equal(t, "Remember the milk", row["description"])
	require.Equal(t, false, row["checked"])
	require.Equal(t, float64(1), row["priority"])
	require.Equal(t, float64(5), row["child_order"])
	require.Equal(t, "2024-01-15T10:00:00Z", row["added_at"])
	require.Equal(t, "u1", row["added_by_uid"])
	require.Equal(t, "u2", row["responsible_uid"])
	require.Equal(t, "u3", row["assigned_by_uid"])
	require.Equal(t, "p1", row["project_id"])
	require.Equal(t, "s1", row["section_id"])
	require.Nil(t, row["parent_id"])
	require.Equal(t, float64(3), row["note_count"])
}

func TestFlattenItem_DueObject(t *testing.T) {
	// Todoist v1 due objects have no separate `datetime` field: `date` holds the
	// date or datetime, and `timezone` is set only for fixed-timezone dates.
	raw := json.RawMessage(`{
		"id":"1",
		"content":"Task",
		"due":{
			"date":"2024-01-15T12:00:00Z",
			"string":"Jan 15",
			"is_recurring":true,
			"timezone":"America/New_York",
			"lang":"en"
		}
	}`)

	row := flattenItem(raw)
	require.Equal(t, "2024-01-15T12:00:00Z", row["dueDate"])
	require.Equal(t, "Jan 15", row["dueString"])
	require.Equal(t, true, row["dueIsRecurring"])
	require.Equal(t, "America/New_York", row["dueTimezone"])
	// v1 has no separate datetime field.
	require.Nil(t, row["dueDateTime"])
}

func TestFlattenItem_DueFullDay(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","due":{"date":"2024-01-15","string":"Jan 15","is_recurring":false,"timezone":null}}`)
	row := flattenItem(raw)
	require.Equal(t, "2024-01-15", row["dueDate"])
	require.Equal(t, false, row["dueIsRecurring"])
	require.Nil(t, row["dueTimezone"])
}

func TestFlattenItem_DueNull(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","due":null}`)
	row := flattenItem(raw)
	require.Nil(t, row["dueDate"])
	require.Nil(t, row["dueIsRecurring"])
}

func TestFlattenItem_Deadline(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","deadline":{"date":"2024-02-12","lang":"en"}}`)
	row := flattenItem(raw)
	require.Equal(t, "2024-02-12", row["deadlineDate"])
}

func TestFlattenItem_DeadlineNull(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","deadline":null}`)
	row := flattenItem(raw)
	require.Nil(t, row["deadlineDate"])
}

func TestFlattenItem_Labels(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","labels":["urgent","work","dev"]}`)
	row := flattenItem(raw)
	require.Equal(t, `["urgent","work","dev"]`, row["labels"])
}

func TestFlattenItem_LabelsEmpty(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","labels":[]}`)
	row := flattenItem(raw)
	require.Nil(t, row["labels"])
}

func TestFlattenItem_DurationObject(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"1","content":"Task",
		"duration":{"amount":30,"unit":"minute"}
	}`)

	row := flattenItem(raw)
	require.Equal(t, int64(30), row["durationAmount"])
	require.Equal(t, "minute", row["durationUnit"])
}

func TestFlattenItem_DurationNull(t *testing.T) {
	raw := json.RawMessage(`{"id":"1","content":"Task","duration":null}`)
	row := flattenItem(raw)
	require.Nil(t, row["durationAmount"])
}

func TestInferColumnType_ISODatesAndScalars(t *testing.T) {
	records := []map[string]any{
		{"added_at": "2024-01-02T03:04:05.000Z"},
		{"added_at": "2024-01-03T03:04:05.000Z"},
	}
	require.Equal(t, fieldTypeTime, inferColumnType("added_at", records))

	// Floating (no timezone) and full-day due dates both infer as time.
	records = []map[string]any{
		{"dueDate": "2024-01-15T12:00:00"},
		{"dueDate": "2024-01-16"},
	}
	require.Equal(t, fieldTypeTime, inferColumnType("dueDate", records))

	// Opaque string IDs stay strings.
	records = []map[string]any{{"id": "6XGgmFVcrG5RRjVr"}, {"id": "6fFPHV272WWh3gpW"}}
	require.Equal(t, fieldTypeString, inferColumnType("id", records))

	records = []map[string]any{{"priority": float64(1)}, {"priority": float64(4)}}
	require.Equal(t, fieldTypeNumber, inferColumnType("priority", records))

	records = []map[string]any{{"checked": true}, {"checked": false}}
	require.Equal(t, fieldTypeBool, inferColumnType("checked", records))
}

func TestToColumnTime_OnlyStrings(t *testing.T) {
	tm, ok := toColumnTime("2024-01-15T10:00:00Z")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), tm.UTC())

	// Floating datetime with microseconds (no zone) parses to UTC.
	tm, ok = toColumnTime("2024-01-15T12:00:00.000000")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), tm.UTC())

	_, ok = toColumnTime(float64(3))
	require.False(t, ok)
}

func TestRecordsToFrame_TimeColumnsFirstAndRowOrderPreserved(t *testing.T) {
	records := []map[string]any{
		{"content": "b", "added_at": "2024-01-03T00:00:00Z"},
		{"content": "a", "added_at": "2024-01-02T00:00:00Z"},
	}
	frame := recordsToFrame("A", records)
	require.Len(t, frame.Fields, 2)
	require.Equal(t, "added_at", frame.Fields[0].Name)
	require.Equal(t, "content", frame.Fields[1].Name)
	require.Equal(t, "b", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(1).(*string))
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("A", nil)
	require.Empty(t, frame.Fields)
}

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, int64(42), frame.Fields[0].At(0).(int64))
}
