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
		{"id": float64(1), "title": "first", "active": true, "add_time": "2024-01-02 03:04:05"},
		{"id": float64(2), "title": "second", "active": false, "add_time": "2024-02-03 04:05:06"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"title": "a", "add_time": "2024-02-03 04:05:06", "value": float64(1)},
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
		{"title": "b", "add_time": "2024-02-03 04:05:06"},
		{"title": "a", "add_time": "2024-01-02 03:04:05"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Equal(t, "add_time", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"title": "c", "add_time": "2024-03-04 05:06:07"},
		{"title": "b", "add_time": "2024-02-03 04:05:06"},
		{"title": "a", "add_time": "2024-01-02 03:04:05"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Equal(t, "add_time", frame.Fields[0].Name)
	require.Equal(t, "title", frame.Fields[1].Name)
	require.Equal(t, "c", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "b", *frame.Fields[1].At(1).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(2).(*string))
}

func TestRecordsToFrame_FieldsSelection(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "title": "Deal", "value": float64(100)},
	}
	frame := recordsToFrame("A", records, []string{"title", "value"})
	require.Len(t, frame.Fields, 2)
	idField, _ := frame.FieldByName("id")
	require.Nil(t, idField)
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

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Equal(t, "A", frame.RefID)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, data.FrameTypeNumericWide, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
	require.Equal(t, data.FieldTypeInt64, frame.Fields[0].Type())
}

func TestToTime_PipedriveFormats(t *testing.T) {
	cases := []string{
		"2024-01-15 10:00:00",      // v1 datetime (UTC, space separated)
		"2024-01-15",               // date only
		"2024-01-15T10:00:00Z",     // RFC3339
		"2024-01-15T10:00:00.123Z", // RFC3339 with millis
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}
	_, ok := toTime("just text")
	require.False(t, ok)
	// Time-of-day values must NOT parse as a date/time instant.
	_, ok = toTime("16:00:00")
	require.False(t, ok)
}

func TestToColumnTime_OnlyDateNamedColumns(t *testing.T) {
	_, ok := toColumnTime("add_time", "2024-01-15 10:00:00")
	require.True(t, ok)
	_, ok = toColumnTime("expected_close_date", "2024-01-15")
	require.True(t, ok)
	// A non-date column with a date-like value stays a string.
	_, ok = toColumnTime("title", "2024-01-15")
	require.False(t, ok)
	// next_activity_time holds HH:MM:SS, which must not become a time instant.
	_, ok = toColumnTime("next_activity_time", "16:00:00")
	require.False(t, ok)
}

func TestInferColumnType(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("value", []map[string]any{{"value": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("active", []map[string]any{{"active": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("add_time", []map[string]any{{"add_time": "2024-01-01 00:00:00"}}))
	require.Equal(t, fieldTypeString, inferColumnType("title", []map[string]any{{"title": "x"}}))
}

func TestFlattenRecord_NestedRelationAndArrays(t *testing.T) {
	raw := json.RawMessage(`{
		"id":1,
		"title":"Deal",
		"person_id":{"name":"Jane Doe","value":42},
		"email":[{"label":"work","value":"jane@example.com","primary":true}],
		"value":1000
	}`)
	rec := flattenRecord(raw)
	require.Equal(t, float64(1), rec["id"])
	require.Equal(t, "Deal", rec["title"])
	require.Equal(t, "Jane Doe", rec["person_id"])     // relation -> name
	require.Equal(t, "jane@example.com", rec["email"]) // array of {value} -> value
	require.Equal(t, float64(1000), rec["value"])
}

func TestFlattenRecord_NullValues(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"close_time":null,"value":null}`)
	rec := flattenRecord(raw)
	require.Equal(t, float64(1), rec["id"])
	require.Nil(t, rec["close_time"])
	require.Nil(t, rec["value"])
}

func TestRemapCustomFields_NoClobber(t *testing.T) {
	records := []map[string]any{
		{sampleHash: "VIP", "Tier": "existing"},
	}
	remapCustomFields(records, map[string]string{sampleHash: "Tier"})
	// Existing "Tier" must not be clobbered; the hash key is retained.
	require.Equal(t, "existing", records[0]["Tier"])
	require.Equal(t, "VIP", records[0][sampleHash])
}

func TestFlattenRecord_FlowsThroughFrame(t *testing.T) {
	records := []map[string]any{
		flattenRecord(json.RawMessage(`{"id":1,"title":"A","value":10}`)),
		flattenRecord(json.RawMessage(`{"id":2,"title":"B","value":20}`)),
	}
	frame := recordsToFrame("A", records, nil)
	valueField, _ := frame.FieldByName("value")
	require.NotNil(t, valueField)
	require.Equal(t, data.FieldTypeNullableFloat64, valueField.Type())
	require.EqualValues(t, 10, *valueField.At(0).(*float64))
}
