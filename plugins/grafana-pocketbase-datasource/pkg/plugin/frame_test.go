package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypesAndIdentityColumnsFirst(t *testing.T) {
	records := []map[string]any{
		{"id": "r1", "Title": "first", "Active": true, "Age": float64(1)},
		{"id": "r2", "Title": "second", "Active": false, "Age": float64(2)},
	}

	frame := recordsToFrame("A", records, false)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	// id leads (no time columns here), followed by alphabetical fields.
	require.Equal(t, "id", frame.Fields[0].Name)

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 2, rowLen)
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"name": "a", "when": "2024-02-03 04:05:06.000Z", "age": float64(1)},
		{"name": "b", "when": "2024-01-02 03:04:05.000Z", "age": float64(2)},
	}
	frame := recordsToFrame("A", records, false)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "when": "2024-02-03 04:05:06.000Z"},
		{"name": "a", "when": "2024-01-02 03:04:05.000Z"},
	}
	frame := recordsToFrame("A", records, false)

	require.Equal(t, "when", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_CreatedParsedAsTime(t *testing.T) {
	records := []map[string]any{
		{"id": "r1", "created": "2024-01-01 12:00:00.000Z", "name": "a"},
	}
	frame := recordsToFrame("A", records, false)
	created, _ := frame.FieldByName("created")
	require.NotNil(t, created)
	require.Equal(t, data.FieldTypeNullableTime, created.Type())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	// PocketBase returns records in sort/default order; the frame layer must not
	// re-sort. Here the records are in descending-Age order and must survive.
	records := []map[string]any{
		{"Age": float64(40), "Name": "c"},
		{"Age": float64(30), "Name": "a"},
		{"Age": float64(20), "Name": "b"},
	}
	frame := recordsToFrame("A", records, false)

	ageField, _ := frame.FieldByName("Age")
	require.NotNil(t, ageField)
	require.EqualValues(t, 40, *ageField.At(0).(*float64))
	require.EqualValues(t, 30, *ageField.At(1).(*float64))
	require.EqualValues(t, 20, *ageField.At(2).(*float64))
}

func TestRecordsToFrame_SpaceDateTimeNormalisedToUTC(t *testing.T) {
	records := []map[string]any{
		{"ts": "2024-01-01 12:00:00.000+05:30"},
	}
	frame := recordsToFrame("A", records, false)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC)
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
}

func TestRecordsToFrame_ArraysAndObjectsSerialiseToJSON(t *testing.T) {
	// PocketBase multi-relations / multi-selects / file lists are arrays.
	records := []map[string]any{
		{"tags": []any{"a", "b"}},
	}
	frame := recordsToFrame("A", records, false)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["a","b"]`, *frame.Fields[0].At(0).(*string))
}

func TestRecordsToFrame_HideSystemFields(t *testing.T) {
	records := []map[string]any{
		{
			"id":             "r1",
			"collectionId":   "c1",
			"collectionName": "posts",
			"created":        "2024-01-01 00:00:00.000Z",
			"updated":        "2024-01-02 00:00:00.000Z",
			"title":          "Alice",
			"score":          float64(5),
		},
	}

	// With the flag off, all columns (including system) are present.
	full := recordsToFrame("A", records, false)
	require.NotNil(t, mustField(full, "id"))
	require.NotNil(t, mustField(full, "collectionName"))
	require.NotNil(t, mustField(full, "title"))

	// With the flag on, only the user fields remain.
	hidden := recordsToFrame("A", records, true)
	require.Equal(t, 2, len(hidden.Fields))
	require.NotNil(t, mustField(hidden, "title"))
	require.NotNil(t, mustField(hidden, "score"))
	for _, f := range hidden.Fields {
		require.False(t, systemColumns[f.Name], "unexpected system column %q", f.Name)
	}
}

func TestDropSystemColumns(t *testing.T) {
	in := []string{"id", "title", "collectionId", "score", "created", "updated", "collectionName"}
	require.Equal(t, []string{"title", "score"}, dropSystemColumns(in))
}

func mustField(frame *data.Frame, name string) *data.Field {
	f, _ := frame.FieldByName(name)
	return f
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("B", nil, false)
	require.Equal(t, "B", frame.RefID)
	require.Equal(t, 0, len(frame.Fields))
}

func TestRecordsToFrame_MixedTypesFallBackToString(t *testing.T) {
	records := []map[string]any{
		{"val": float64(1)},
		{"val": "text"},
	}
	frame := recordsToFrame("C", records, false)
	require.Equal(t, 1, len(frame.Fields))
	_, ok := frame.Fields[0].At(0).(*string)
	require.True(t, ok)
}

func TestInferColumnType(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("b", []map[string]any{{"b": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01 00:00:00.000Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime_PocketBaseDateTimeFormats(t *testing.T) {
	cases := []string{
		"2026-06-15 09:30:55.052Z",      // PocketBase autodate (space separator, millis, UTC)
		"2026-06-15 09:30:55Z",          // no fraction
		"2026-06-15 09:30:55.000+05:30", // offset
		"2024-01-01T00:00:00Z",          // RFC3339 (ISO with T)
		"2024-01-01",                    // date only
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

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 1, rowLen)
}
