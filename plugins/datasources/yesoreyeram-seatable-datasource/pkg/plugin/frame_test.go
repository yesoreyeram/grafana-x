package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("B", nil)
	require.Equal(t, "B", frame.RefID)
	require.Empty(t, frame.Fields)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
}

func TestRecordsToFrame_IdentityColumnsFirst(t *testing.T) {
	records := []map[string]any{
		{"_id": "r1", "_ctime": "2024-01-01T00:00:00.000+00:00", "_mtime": "2024-01-02T00:00:00.000+00:00", "Name": "Alice", "Age": float64(30)},
		{"_id": "r2", "_ctime": "2024-02-01T00:00:00.000+00:00", "_mtime": "2024-02-02T00:00:00.000+00:00", "Name": "Bob", "Age": float64(25)},
	}
	frame := recordsToFrame("A", records)
	names := make([]string, len(frame.Fields))
	for i, f := range frame.Fields {
		names[i] = f.Name
	}
	// Time identity columns lead, then _id, then user columns alphabetically.
	require.Equal(t, []string{"_ctime", "_mtime", "_id", "Age", "Name"}, names)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"Name": "a", "When": "2024-02-03T04:05:06.000Z", "Age": float64(1)},
	}
	frame := recordsToFrame("A", records)
	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_InfersTypes(t *testing.T) {
	records := []map[string]any{
		{"name": "Alice", "age": float64(30), "active": true, "created": "2024-01-15T10:00:00Z"},
		{"name": "Bob", "age": float64(25), "active": false, "created": "2024-02-20T14:30:00Z"},
	}
	frame := recordsToFrame("A", records)
	byName := map[string]*data.Field{}
	for _, f := range frame.Fields {
		byName[f.Name] = f
	}
	require.Equal(t, data.FieldTypeNullableString, byName["name"].Type())
	require.Equal(t, data.FieldTypeNullableFloat64, byName["age"].Type())
	require.Equal(t, data.FieldTypeNullableBool, byName["active"].Type())
	require.Equal(t, data.FieldTypeNullableTime, byName["created"].Type())
	require.Equal(t, "created", frame.Fields[0].Name) // time first
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"Age": float64(40), "Name": "c"},
		{"Age": float64(30), "Name": "a"},
		{"Age": float64(20), "Name": "b"},
	}
	frame := recordsToFrame("A", records)
	ageField, _ := frame.FieldByName("Age")
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
	// SeaTable multiple-select / collaborator / link cells are arrays/objects.
	records := []map[string]any{
		{"Tags": []any{"a", "b"}},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["a","b"]`, *frame.Fields[0].At(0).(*string))
}

func TestRecordsToFrame_MixedTypesFallBackToString(t *testing.T) {
	records := []map[string]any{
		{"val": float64(1)},
		{"val": "text"},
	}
	frame := recordsToFrame("C", records)
	require.Len(t, frame.Fields, 1)
	_, ok := frame.Fields[0].At(0).(*string)
	require.True(t, ok)
}

func TestInferColumnType(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("b", []map[string]any{{"b": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00.000+00:00"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": nil}, {"n": float64(1)}}))
}

func TestToTime_SeaTableFormats(t *testing.T) {
	cases := []string{
		"2025-09-15T10:57:19.106+02:00", // ctime with millis + offset
		"2025-09-18T09:52:00+00:00",     // mtime, no millis, offset
		"2024-01-15T10:00:00Z",          // RFC3339
		"2020-09-08 00:11:23",           // SQL time constant
		"2024-01-15",                    // date only
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}
	_, ok := toTime("not a time")
	require.False(t, ok)
	_, ok = toTime(float64(2024))
	require.False(t, ok)
}

func TestOrderedColumns_IdentityFirstThenAlpha(t *testing.T) {
	records := []map[string]any{
		{"b": 1, "a": 2, "_id": "x", "_mtime": "t"},
	}
	cols := orderedColumns(records)
	require.Equal(t, []string{"_id", "_mtime", "a", "b"}, cols)
}

func TestOrderTimeFirst(t *testing.T) {
	colTypes := map[string]fieldType{"name": fieldTypeString, "ts": fieldTypeTime, "age": fieldTypeNumber}
	require.Equal(t, []string{"ts", "name", "age"}, orderTimeFirst([]string{"name", "ts", "age"}, colTypes))
}

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Equal(t, "A", frame.RefID)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeInt64, frame.Fields[0].Type())
	require.Equal(t, data.FrameTypeNumericWide, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
}

func TestToStringHelpers(t *testing.T) {
	s, ok := toString("hello")
	require.True(t, ok)
	require.Equal(t, "hello", s)
	s, ok = toString(float64(42))
	require.True(t, ok)
	require.Equal(t, "42", s)
	_, ok = toString(nil)
	require.False(t, ok)
}
