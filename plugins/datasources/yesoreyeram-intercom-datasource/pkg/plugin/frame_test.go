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
		{"id": "1", "name": "first", "active": true, "created_at": "2024-01-02T03:04:05Z"},
		{"id": "2", "name": "second", "active": false, "created_at": "2024-02-03T04:05:06Z"},
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
		{"name": "a", "created_at": "2024-02-03T04:05:06Z", "age": float64(1)},
		{"name": "b", "created_at": "2024-01-02T03:04:05Z", "age": float64(2)},
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
		{"name": "b", "created_at": "2024-02-03T04:05:06Z"},
		{"name": "a", "created_at": "2024-01-02T03:04:05Z"},
		{"name": "c", "created_at": "2024-03-04T05:06:07Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "created_at", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	// The frame layer must NOT re-sort rows: the order returned by Intercom
	// (which honours the query sort) must be preserved exactly.
	records := []map[string]any{
		{"name": "c", "created_at": "2024-03-04T05:06:07Z"},
		{"name": "b", "created_at": "2024-02-03T04:05:06Z"},
		{"name": "a", "created_at": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "created_at", frame.Fields[0].Name)
	require.Equal(t, "name", frame.Fields[1].Name)

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

// ---------------------------------------------------------------------------
// Intercom record flattening
// ---------------------------------------------------------------------------

func TestFlattenIntercomRecord_ConvertsEpochSecondsToTime(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"contact","id":"abc","email":"a@b.com",
		"created_at":1394539050,
		"signed_up_at":1394539050,
		"last_seen_at":0
	}`)
	row := flattenIntercomRecord(raw, 0)
	require.Equal(t, "abc", row["id"])
	require.Equal(t, "a@b.com", row["email"])
	require.Equal(t, "2014-03-11T11:57:30Z", row["created_at"])
	require.Equal(t, "2014-03-11T11:57:30Z", row["signed_up_at"])
	// 0 epoch means "unset" and becomes null.
	require.Nil(t, row["last_seen_at"])
}

func TestFlattenIntercomRecord_SerialisesNestedObjects(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"1",
		"source":{"type":"conversation","author":{"type":"user","email":"x@y.com"}},
		"tags":{"type":"tag.list","tags":[{"id":"7","name":"vip"}]},
		"custom_attributes":{"plan":"pro"}
	}`)
	row := flattenIntercomRecord(raw, 0)
	require.Equal(t, "1", row["id"])
	// Nested objects become compact JSON strings (keys sorted deterministically).
	require.Equal(t, `{"author":{"email":"x@y.com","type":"user"},"type":"conversation"}`, row["source"])
	require.Equal(t, `{"plan":"pro"}`, row["custom_attributes"])
	require.Contains(t, row["tags"].(string), `"name":"vip"`)
}

func TestFlattenIntercomRecord_AddsSyntheticID(t *testing.T) {
	raw := json.RawMessage(`{"type":"tag","name":"vip"}`)
	row := flattenIntercomRecord(raw, 4)
	require.Equal(t, "row-4", row["id"])
}

func TestFlattenIntercomRecord_FlowsThroughFrame(t *testing.T) {
	rows := []map[string]any{
		flattenIntercomRecord(json.RawMessage(`{"id":"1","state":"open","created_at":1539897198}`), 0),
		flattenIntercomRecord(json.RawMessage(`{"id":"2","state":"closed","created_at":1539900000}`), 1),
	}
	frame := recordsToFrame("A", rows)

	created, _ := frame.FieldByName("created_at")
	require.NotNil(t, created)
	require.Equal(t, data.FieldTypeNullableTime, created.Type())

	state, _ := frame.FieldByName("state")
	require.NotNil(t, state)
	require.Equal(t, "open", *state.At(0).(*string))
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
