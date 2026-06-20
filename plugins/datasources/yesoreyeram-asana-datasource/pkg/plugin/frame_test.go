package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlattenItem_ReducesNestedRelations(t *testing.T) {
	raw := json.RawMessage(`{
		"gid":"1",
		"name":"Fix login bug",
		"completed":false,
		"assignee":{"gid":"10","name":"Alice"},
		"parent":{"gid":"9","name":"Epic"},
		"projects":[{"gid":"p1","name":"Mobile"},{"gid":"p2","name":"Web"}],
		"tags":[{"gid":"t1","name":"bug"},{"gid":"t2","name":"p1"}],
		"followers":[{"gid":"10","name":"Alice"},{"gid":"11","name":"Bob"}],
		"created_at":"2024-01-02T03:04:05.000Z",
		"due_on":"2024-02-01",
		"due_at":null
	}`)

	row := flattenItem(raw)
	require.Equal(t, "1", row["gid"])
	require.Equal(t, "Fix login bug", row["name"])
	require.Equal(t, false, row["completed"])
	require.Equal(t, "Alice", row["assignee"])
	require.Equal(t, "Epic", row["parent"])
	require.Equal(t, "Mobile, Web", row["projects"])
	require.Equal(t, "bug, p1", row["tags"])
	require.Equal(t, "Alice, Bob", row["followers"])
	require.Equal(t, "2024-01-02T03:04:05.000Z", row["created_at"])
	require.Equal(t, "2024-02-01", row["due_on"])
	require.Nil(t, row["due_at"])
}

func TestFlattenItem_CustomFields(t *testing.T) {
	raw := json.RawMessage(`{
		"gid":"1","name":"Task",
		"custom_fields":[
			{"gid":"cf1","name":"Priority","type":"enum","enum_value":{"name":"High"},"display_value":"High"},
			{"gid":"cf2","name":"Story Points","type":"number","number_value":5,"display_value":"5"},
			{"gid":"cf3","name":"Sprint","type":"text","text_value":"S12","display_value":"S12"},
			{"gid":"cf4","name":"Labels","type":"multi_enum","multi_enum_values":[{"name":"a"},{"name":"b"}],"display_value":"a, b"},
			{"gid":"cf5","name":"Target","type":"date","date_value":{"date":"2024-05-01","date_time":null},"display_value":"2024-05-01"},
			{"gid":"cf6","name":"Empty","type":"enum","enum_value":null,"display_value":null}
		]
	}`)
	row := flattenItem(raw)
	require.Equal(t, "High", row["Priority"])
	require.Equal(t, float64(5), row["Story Points"]) // numeric value preserved
	require.Equal(t, "S12", row["Sprint"])
	require.Equal(t, "a, b", row["Labels"])
	require.Equal(t, "2024-05-01", row["Target"])
	require.Nil(t, row["Empty"])
	// custom_fields itself is expanded, not kept as a raw column.
	require.NotContains(t, row, "custom_fields")
}

func TestFlattenItem_CustomFieldNameCollisionSuffixed(t *testing.T) {
	raw := json.RawMessage(`{
		"gid":"1","name":"Real Task",
		"custom_fields":[{"gid":"cf1","name":"name","type":"text","text_value":"v","display_value":"v"}]
	}`)
	row := flattenItem(raw)
	require.Equal(t, "Real Task", row["name"]) // standard field preserved
	require.Equal(t, "v", row["name (2)"])     // colliding custom field suffixed
}

func TestFlattenItem_CurrentStatusText(t *testing.T) {
	row := flattenItem(json.RawMessage(`{"gid":"p1","current_status":{"text":"On track","color":"green"}}`))
	require.Equal(t, "On track", row["current_status"])

	row = flattenItem(json.RawMessage(`{"gid":"p2","current_status":null}`))
	require.Nil(t, row["current_status"])
}

func TestInferColumnType_ISODatesAndScalars(t *testing.T) {
	records := []map[string]any{
		{"created_at": "2024-01-02T03:04:05.000Z"},
		{"created_at": "2024-01-03T03:04:05.000Z"},
	}
	require.Equal(t, fieldTypeTime, inferColumnType("created_at", records))

	// date-only values are also time.
	records = []map[string]any{{"due_on": "2024-02-01"}, {"due_on": "2024-03-01"}}
	require.Equal(t, fieldTypeTime, inferColumnType("due_on", records))

	// numeric gid strings stay strings (not numbers, not time).
	records = []map[string]any{{"gid": "1001"}, {"gid": "1002"}}
	require.Equal(t, fieldTypeString, inferColumnType("gid", records))

	// numbers stay numeric.
	records = []map[string]any{{"num_subtasks": float64(3)}, {"num_subtasks": float64(0)}}
	require.Equal(t, fieldTypeNumber, inferColumnType("num_subtasks", records))

	// booleans stay boolean.
	records = []map[string]any{{"completed": true}, {"completed": false}}
	require.Equal(t, fieldTypeBool, inferColumnType("completed", records))
}

func TestToColumnTime_OnlyStrings(t *testing.T) {
	tm, ok := toColumnTime("2024-01-02T03:04:05Z")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), tm.UTC())

	// A bare number is not a time.
	_, ok = toColumnTime(float64(3))
	require.False(t, ok)
}

func TestRecordsToFrame_TimeColumnsFirstAndRowOrderPreserved(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "created_at": "2024-01-03T00:00:00Z"},
		{"name": "a", "created_at": "2024-01-02T00:00:00Z"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Len(t, frame.Fields, 2)
	// Time column should be first.
	require.Equal(t, "created_at", frame.Fields[0].Name)
	require.Equal(t, "name", frame.Fields[1].Name)
	// Row order preserved (NOT re-sorted): "b" then "a".
	require.Equal(t, "b", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(1).(*string))
}

func TestRecordsToFrame_FieldSelection(t *testing.T) {
	records := []map[string]any{{"gid": "1", "name": "x", "notes": "hello"}}
	frame := recordsToFrame("A", records, []string{"name"})
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "name", frame.Fields[0].Name)
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("A", nil, nil)
	require.Empty(t, frame.Fields)
}

func TestSelectColumns(t *testing.T) {
	cols := []string{"gid", "name", "notes"}
	require.Equal(t, []string{"name", "notes"}, selectColumns(cols, []string{"notes", "name", "missing"}))
	require.Equal(t, cols, selectColumns(cols, nil))
	require.Equal(t, cols, selectColumns(cols, []string{"   "}))
}
