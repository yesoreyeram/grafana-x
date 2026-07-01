package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypes(t *testing.T) {
	records := []map[string]any{
		{"name": "Alice", "age": float64(30), "active": true, "created": "2024-01-15T10:00:00.000Z"},
		{"name": "Bob", "age": float64(25), "active": false, "created": "2024-02-20T14:30:00.000Z"},
	}
	frame := recordsToFrame("A", records)
	require.NotNil(t, frame)

	fields := map[string]*data.Field{}
	for _, f := range frame.Fields {
		fields[f.Name] = f
	}
	require.Equal(t, data.FieldTypeNullableString, fields["name"].Type())
	require.Equal(t, data.FieldTypeNullableFloat64, fields["age"].Type())
	require.Equal(t, data.FieldTypeNullableBool, fields["active"].Type())
	require.Equal(t, data.FieldTypeNullableTime, fields["created"].Type())

	// Time field is moved to the front.
	require.Equal(t, "created", frame.Fields[0].Name)
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"Name": "a", "When": "2024-02-03T04:05:06.000Z", "Age": float64(1)},
		{"Name": "b", "When": "2024-01-02T03:04:05.000Z", "Age": float64(2)},
	}
	frame := recordsToFrame("A", records)
	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_IdentityAndTimeColumnsOrder(t *testing.T) {
	records := []map[string]any{
		{
			"_id":               "rec1",
			"_createdTime":      "2024-01-01T00:00:00.000Z",
			"_lastModifiedTime": "2024-02-01T00:00:00.000Z",
			"Name":              "Alice",
			"Age":               float64(30),
		},
	}
	frame := recordsToFrame("A", records)

	// Time identity columns (_createdTime, _lastModifiedTime) come first, then
	// the _id identity column, then user fields alphabetically.
	names := []string{}
	for _, f := range frame.Fields {
		names = append(names, f.Name)
	}
	require.Equal(t, []string{"_createdTime", "_lastModifiedTime", "_id", "Age", "Name"}, names)

	_, idIdx := frame.FieldByName("_id")
	require.NotEqual(t, -1, idIdx)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	// Teable returns records in sort/view order; the frame layer must not re-sort.
	records := []map[string]any{
		{"Age": float64(40), "Name": "c"},
		{"Age": float64(30), "Name": "a"},
		{"Age": float64(20), "Name": "b"},
	}
	frame := recordsToFrame("A", records)
	ageField, idx := frame.FieldByName("Age")
	require.NotEqual(t, -1, idx)
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
	// Teable multipleSelect / link / attachment / user cells are arrays/objects.
	records := []map[string]any{
		{"Tags": []any{"a", "b"}},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["a","b"]`, *frame.Fields[0].At(0).(*string))
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("B", nil)
	require.Equal(t, "B", frame.RefID)
	require.Empty(t, frame.Fields)
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
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00.000Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
	require.Equal(t, fieldTypeNumber, inferColumnType("x", []map[string]any{{"x": nil}, {"x": float64(42)}}))
}

func TestToTime_TeableFormats(t *testing.T) {
	for _, c := range []string{
		"2024-09-02T02:51:03.875Z", // ISO with millis, UTC (Teable default)
		"2024-01-01T00:00:00Z",     // RFC3339
		"2024-01-01T12:00:00.000+05:30",
		"2024-01-15", // date only
	} {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}
	_, ok := toTime("not a date")
	require.False(t, ok)
}

func TestToString(t *testing.T) {
	require.Equal(t, "hello", mustToString("hello"))
	require.Equal(t, "42", mustToString(float64(42)))
	require.Empty(t, mustToString(nil))
	require.Equal(t, `[1,2]`, mustToString([]any{float64(1), float64(2)}))
}

func mustToString(v any) string {
	s, ok := toString(v)
	if !ok {
		return ""
	}
	return s
}

func TestOrderedColumns_IdentityFirst(t *testing.T) {
	records := []map[string]any{
		{"b": 1, "a": 2, "_id": "rec1"},
		{"c": 3},
	}
	cols := orderedColumns(records)
	require.Equal(t, []string{"_id", "a", "b", "c"}, cols)
}

func TestOrderTimeFirst(t *testing.T) {
	colTypes := map[string]fieldType{
		"name": fieldTypeString,
		"ts":   fieldTypeTime,
		"age":  fieldTypeNumber,
	}
	require.Equal(t, []string{"ts", "name", "age"}, orderTimeFirst([]string{"name", "ts", "age"}, colTypes))
}

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Equal(t, "A", frame.RefID)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeInt64, frame.Fields[0].Type())
	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeNumericWide, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
}
