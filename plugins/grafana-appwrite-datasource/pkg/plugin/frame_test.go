package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestDocumentsToFrame_InfersTypesAndIdentityColumnsFirst(t *testing.T) {
	documents := []map[string]any{
		{"$id": "d1", "Title": "first", "Active": true, "Age": float64(1)},
		{"$id": "d2", "Title": "second", "Active": false, "Age": float64(2)},
	}

	frame := documentsToFrame("A", documents, false)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	// $id leads (no time columns here), followed by alphabetical fields.
	require.Equal(t, "$id", frame.Fields[0].Name)

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 2, rowLen)
}

func TestDocumentsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	documents := []map[string]any{
		{"name": "a", "when": "2024-02-03T04:05:06.000Z", "age": float64(1)},
		{"name": "b", "when": "2024-01-02T03:04:05.000Z", "age": float64(2)},
	}
	frame := documentsToFrame("A", documents, false)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestDocumentsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	documents := []map[string]any{
		{"name": "b", "when": "2024-02-03T04:05:06.000Z"},
		{"name": "a", "when": "2024-01-02T03:04:05.000Z"},
	}
	frame := documentsToFrame("A", documents, false)

	require.Equal(t, "when", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestDocumentsToFrame_CreatedAtParsedAsTime(t *testing.T) {
	documents := []map[string]any{
		{"$id": "d1", "$createdAt": "2024-01-01T12:00:00.000+00:00", "name": "a"},
	}
	frame := documentsToFrame("A", documents, false)
	createdAt, _ := frame.FieldByName("$createdAt")
	require.NotNil(t, createdAt)
	require.Equal(t, data.FieldTypeNullableTime, createdAt.Type())
}

func TestDocumentsToFrame_PreservesRowOrder(t *testing.T) {
	// Appwrite returns documents in sort/default order; the frame layer must not
	// re-sort. Here the documents are in descending-Age order and must survive.
	documents := []map[string]any{
		{"Age": float64(40), "Name": "c"},
		{"Age": float64(30), "Name": "a"},
		{"Age": float64(20), "Name": "b"},
	}
	frame := documentsToFrame("A", documents, false)

	ageField, _ := frame.FieldByName("Age")
	require.NotNil(t, ageField)
	require.EqualValues(t, 40, *ageField.At(0).(*float64))
	require.EqualValues(t, 30, *ageField.At(1).(*float64))
	require.EqualValues(t, 20, *ageField.At(2).(*float64))
}

func TestDocumentsToFrame_OffsetDateTimeNormalisedToUTC(t *testing.T) {
	documents := []map[string]any{
		{"ts": "2024-01-01T12:00:00.000+05:30"},
	}
	frame := documentsToFrame("A", documents, false)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC)
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
}

func TestDocumentsToFrame_ArraysAndObjectsSerialiseToJSON(t *testing.T) {
	// Appwrite array attributes / $permissions / relationships are arrays/objects.
	documents := []map[string]any{
		{"tags": []any{"a", "b"}},
	}
	frame := documentsToFrame("A", documents, false)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["a","b"]`, *frame.Fields[0].At(0).(*string))
}

func TestDocumentsToFrame_HideSystemFields(t *testing.T) {
	documents := []map[string]any{
		{
			"$id":           "d1",
			"$createdAt":    "2024-01-01T00:00:00.000+00:00",
			"$updatedAt":    "2024-01-02T00:00:00.000+00:00",
			"$collectionId": "col1",
			"$databaseId":   "db1",
			"$permissions":  []any{`read("any")`},
			"$sequence":     float64(1),
			"name":          "Alice",
			"Gender":        "F",
		},
	}

	// With the flag off, all columns (including $-prefixed) are present.
	full := documentsToFrame("A", documents, false)
	require.NotNil(t, mustField(full, "$id"))
	require.NotNil(t, mustField(full, "$permissions"))
	require.NotNil(t, mustField(full, "name"))

	// With the flag on, only the user attributes remain.
	hidden := documentsToFrame("A", documents, true)
	require.Equal(t, 2, len(hidden.Fields))
	require.NotNil(t, mustField(hidden, "name"))
	require.NotNil(t, mustField(hidden, "Gender"))
	for _, f := range hidden.Fields {
		require.False(t, len(f.Name) > 0 && f.Name[0] == '$', "unexpected system column %q", f.Name)
	}
}

func TestDropSystemColumns(t *testing.T) {
	in := []string{"$id", "name", "$permissions", "Gender", "$sequence"}
	require.Equal(t, []string{"name", "Gender"}, dropSystemColumns(in))
}

func mustField(frame *data.Frame, name string) *data.Field {
	f, _ := frame.FieldByName(name)
	return f
}

func TestDocumentsToFrame_Empty(t *testing.T) {
	frame := documentsToFrame("B", nil, false)
	require.Equal(t, "B", frame.RefID)
	require.Equal(t, 0, len(frame.Fields))
}

func TestDocumentsToFrame_MixedTypesFallBackToString(t *testing.T) {
	documents := []map[string]any{
		{"val": float64(1)},
		{"val": "text"},
	}
	frame := documentsToFrame("C", documents, false)
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

func TestToTime_AppwriteDateTimeFormats(t *testing.T) {
	cases := []string{
		"2026-06-15T09:30:55.000+00:00", // Appwrite system datetime
		"2026-06-15T09:30:55.000Z",      // datetime with millis, UTC
		"2026-06-15T09:30:55.000+05:30", // datetime with millis, offset
		"2024-01-01T00:00:00Z",          // RFC3339
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
