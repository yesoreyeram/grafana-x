package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypes(t *testing.T) {
	records := []map[string]any{
		{"name": "Alice", "active": true, "age": float64(30)},
		{"name": "Bob", "active": false, "age": float64(25)},
	}

	frame := recordsToFrame("A", records)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 3, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"name": "a", "when": "2024-02-03T04:05:06.000Z", "age": float64(1)},
		{"name": "b", "when": "2024-01-02T03:04:05.000Z", "age": float64(2)},
	}
	frame := recordsToFrame("A", records)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
}

func TestRecordsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "when": "2024-02-03T04:05:06.000Z"},
		{"name": "a", "when": "2024-01-02T03:04:05.000Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "when", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"age": float64(40), "name": "c"},
		{"age": float64(30), "name": "a"},
		{"age": float64(20), "name": "b"},
	}
	frame := recordsToFrame("A", records)

	ageField, _ := frame.FieldByName("age")
	require.NotNil(t, ageField)
	require.EqualValues(t, 40, *ageField.At(0).(*float64))
	require.EqualValues(t, 30, *ageField.At(1).(*float64))
	require.EqualValues(t, 20, *ageField.At(2).(*float64))
}

func TestRecordsToFrame_OffsetDateTimeNormalisedToUTC(t *testing.T) {
	records := []map[string]any{
		{"ts": "2024-01-01T12:00:00.000+05:30"},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC)
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
}

func TestRecordsToFrame_ArraysAndObjectsSerialiseToJSON(t *testing.T) {
	records := []map[string]any{
		{"tags": []any{"a", "b"}},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["a","b"]`, *frame.Fields[0].At(0).(*string))
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
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00.000Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime_DateTimeFormats(t *testing.T) {
	cases := []string{
		"2026-06-15T09:30:55.000Z",
		"2026-06-15T09:30:55.000+05:30",
		"2024-01-01T00:00:00Z",
		"2024-01-01",
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}

	_, ok := toTime("request completed")
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
