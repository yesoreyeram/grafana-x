package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlattenEntity_ExpandedRelations(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"550e",
		"name":"Fix login bug",
		"priority":"high",
		"sequence_id":123,
		"state":{"id":"st1","name":"In Progress","group":"started"},
		"assignees":[{"id":"u1","display_name":"Alice"},{"id":"u2","display_name":"Bob"}],
		"labels":[{"id":"l1","name":"bug"},{"id":"l2","name":"p1"}],
		"created_by":{"id":"u9","display_name":"Carol"},
		"project":{"id":"p1","name":"Apollo"},
		"created_at":"2024-01-01T00:00:00Z",
		"target_date":null
	}`)

	row := flattenEntity(raw)
	require.Equal(t, "550e", row["id"])
	require.Equal(t, "Fix login bug", row["name"])
	require.Equal(t, "high", row["priority"])
	require.EqualValues(t, 123, row["sequence_id"])
	require.Equal(t, "In Progress", row["state"])
	require.Equal(t, "started", row["state_group"])
	require.Equal(t, "Alice, Bob", row["assignees"])
	require.Equal(t, "bug, p1", row["labels"])
	require.Equal(t, "Carol", row["created_by"])
	require.Equal(t, "Apollo", row["project"])
	require.Equal(t, "2024-01-01T00:00:00Z", row["created_at"])
	require.Nil(t, row["target_date"])
}

func TestFlattenEntity_UnexpandedRelations(t *testing.T) {
	// When relations are not expanded, Plane returns UUID strings (state) and
	// arrays of UUID strings (assignees/labels). These should pass through.
	raw := json.RawMessage(`{
		"id":"1",
		"state":"st-uuid",
		"assignees":["u1-uuid","u2-uuid"],
		"labels":[],
		"created_by":"u9-uuid"
	}`)
	row := flattenEntity(raw)
	require.Equal(t, "st-uuid", row["state"])
	require.Equal(t, "u1-uuid, u2-uuid", row["assignees"])
	require.Nil(t, row["labels"])
	require.Equal(t, "u9-uuid", row["created_by"])
}

func TestInferColumnType_PlaneDates(t *testing.T) {
	records := []map[string]any{
		{"created_at": "2024-01-01T00:00:00Z"},
		{"created_at": "2024-02-01T12:30:00Z"},
	}
	require.Equal(t, fieldTypeTime, inferColumnType("created_at", records))

	// A plain date string.
	records = []map[string]any{{"target_date": "2024-03-15"}}
	require.Equal(t, fieldTypeTime, inferColumnType("target_date", records))

	// A non-date numeric column stays numeric.
	records = []map[string]any{{"sequence_id": float64(3)}, {"sequence_id": float64(5)}}
	require.Equal(t, fieldTypeNumber, inferColumnType("sequence_id", records))
}

func TestToColumnTime_RFC3339(t *testing.T) {
	tm, ok := toColumnTime("created_at", "2024-01-01T00:00:00Z")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), tm.UTC())

	// A non-date column with a date-like string is not treated as time.
	_, ok = toColumnTime("name", "2024-01-01T00:00:00Z")
	require.False(t, ok)

	// A non-date column with a plain number is not a time.
	_, ok = toColumnTime("sequence_id", float64(3))
	require.False(t, ok)
}

func TestRecordsToFrame_TimeColumnsFirstAndRowOrderPreserved(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "created_at": "2024-02-01T00:00:00Z"},
		{"name": "a", "created_at": "2024-01-01T00:00:00Z"},
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
	records := []map[string]any{{"id": "1", "name": "x", "priority": "high"}}
	frame := recordsToFrame("A", records, []string{"name"})
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "name", frame.Fields[0].Name)
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("A", nil, nil)
	require.Empty(t, frame.Fields)
}

func TestSelectColumns(t *testing.T) {
	cols := []string{"id", "name", "priority"}
	require.Equal(t, []string{"name", "priority"}, selectColumns(cols, []string{"priority", "name", "missing"}))
	require.Equal(t, cols, selectColumns(cols, nil))
	require.Equal(t, cols, selectColumns(cols, []string{"   "}))
}

func TestFlattenListResponse_Variants(t *testing.T) {
	// Bare array.
	recs := flattenListResponse(json.RawMessage(`[{"id":"1"},{"id":"2"}]`))
	require.Len(t, recs, 2)

	// Results envelope.
	recs = flattenListResponse(json.RawMessage(`{"results":[{"id":"a"}]}`))
	require.Len(t, recs, 1)
	require.Equal(t, "a", recs[0]["id"])

	// Single object (no array).
	recs = flattenListResponse(json.RawMessage(`{"id":"solo","name":"x"}`))
	require.Len(t, recs, 1)
	require.Equal(t, "solo", recs[0]["id"])
}
