package plugin

import (
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

	byName := map[string]int{}
	for _, f := range frame.Fields {
		byName[f.Name] = f.Len()
	}
	require.Contains(t, byName, "Id")
	require.Contains(t, byName, "Title")
	require.Contains(t, byName, "Active")
	require.Contains(t, byName, "CreatedAt")

	// All fields must have equal length for a valid frame (RowLen errors otherwise).
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

	// Frame is tagged with the data plane "table" contract.
	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	// Fields all share the same length (valid frame).
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

	// Time field is first and typed as nullable time.
	require.Equal(t, "CreatedAt", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())

	// Time values are stored in UTC.
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	// The frame layer must NOT re-sort rows: the order returned by NocoDB (which
	// honours the query's `sort`) must be preserved exactly. Here the records are
	// in descending-time order and that order must survive.
	records := []map[string]any{
		{"Name": "c", "CreatedAt": "2024-03-04T05:06:07Z"},
		{"Name": "b", "CreatedAt": "2024-02-03T04:05:06Z"},
		{"Name": "a", "CreatedAt": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records)

	// Columns: time first (CreatedAt), then Name.
	require.Equal(t, "CreatedAt", frame.Fields[0].Name)
	require.Equal(t, "Name", frame.Fields[1].Name)

	// Row order is unchanged (still c, b, a — descending by time).
	require.Equal(t, "c", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "b", *frame.Fields[1].At(1).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(2).(*string))

	t0 := frame.Fields[0].At(0).(*time.Time)
	t1 := frame.Fields[0].At(1).(*time.Time)
	t2 := frame.Fields[0].At(2).(*time.Time)
	require.True(t, t0.After(*t1))
	require.True(t, t1.After(*t2))
}

func TestRecordsToFrame_OffsetDateTimeNormalisedToUTC(t *testing.T) {
	// NocoDB returns DateTime with a zone offset; the instant must be preserved
	// but normalised to UTC for the data frame.
	records := []map[string]any{
		{"ts": "2024-01-01 12:00:00+05:30"},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC) // 12:00 +05:30 == 06:30 UTC
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
}

func TestRecordsToFrame_CheckboxStaysNumeric(t *testing.T) {
	// NocoDB Checkbox values come back as 1/0; they should remain numeric (a 0/1
	// column is indistinguishable from a real numeric column without metadata).
	records := []map[string]any{{"Active": float64(1)}, {"Active": float64(0)}}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableFloat64, frame.Fields[0].Type())
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
	// Mixed column should serialise both values as strings (nullable *string).
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

func TestToTime_NocoDBDateTimeFormats(t *testing.T) {
	cases := []string{
		"2026-06-15 09:30:55+00:00", // NocoDB DateTime, UTC
		"2026-06-15 09:30:55+05:30", // NocoDB DateTime, offset zone
		"2024-01-01T00:00:00Z",      // RFC3339
		"2024-01-01 12:00:00",       // naive datetime
		"2024-01-01",                // date only
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}

	// A plain string must not be misread as time.
	_, ok := toTime("request completed")
	require.False(t, ok)
}

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Equal(t, "A", frame.RefID)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, 1, frame.Fields[0].Len())

	// Conforms to the data plane "numeric wide" contract.
	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeNumericWide, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
	require.Equal(t, data.FieldTypeInt64, frame.Fields[0].Type())

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 1, rowLen)
}
