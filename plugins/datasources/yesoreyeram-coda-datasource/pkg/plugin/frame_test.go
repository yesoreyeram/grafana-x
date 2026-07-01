package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRowsToFrame_InfersTypesAndIdentityColumnsFirst(t *testing.T) {
	records := []map[string]any{
		{"id": "row1", "name": "Row 1", "href": "https://coda.io/row1", "Title": "first", "Active": true, "Score": float64(1)},
		{"id": "row2", "name": "Row 2", "href": "https://coda.io/row2", "Title": "second", "Active": false, "Score": float64(2)},
	}

	frame := rowsToFrame("A", records)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 6, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	// Identity columns first (no time columns here), then data columns.
	require.Equal(t, "id", frame.Fields[0].Name)
	require.Equal(t, "name", frame.Fields[1].Name)
	require.Equal(t, "href", frame.Fields[2].Name)
}

func TestRowsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"Name": "a", "When": "2024-02-03T04:05:06.000Z", "Score": float64(1)},
	}
	frame := rowsToFrame("A", records)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
}

func TestRowsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	records := []map[string]any{
		{"Name": "b", "When": "2024-02-03T04:05:06.000Z"},
		{"Name": "a", "When": "2024-01-02T03:04:05.000Z"},
	}
	frame := rowsToFrame("A", records)

	require.Equal(t, "When", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRowsToFrame_TimeMetadataColumnsMovedFront(t *testing.T) {
	records := []map[string]any{
		{"id": "r1", "name": "n1", "createdAt": "2024-01-01T00:00:00.000Z", "updatedAt": "2024-02-01T00:00:00.000Z", "Score": float64(1)},
	}
	frame := rowsToFrame("A", records)
	// createdAt/updatedAt are time-typed and pulled to the front.
	require.Equal(t, "createdAt", frame.Fields[0].Name)
	require.Equal(t, "updatedAt", frame.Fields[1].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
}

func TestRowsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"Score": float64(40), "Name": "c"},
		{"Score": float64(30), "Name": "a"},
		{"Score": float64(20), "Name": "b"},
	}
	frame := rowsToFrame("A", records)

	scoreField, _ := frame.FieldByName("Score")
	require.NotNil(t, scoreField)
	require.EqualValues(t, 40, *scoreField.At(0).(*float64))
	require.EqualValues(t, 30, *scoreField.At(1).(*float64))
	require.EqualValues(t, 20, *scoreField.At(2).(*float64))
}

func TestRowsToFrame_Empty(t *testing.T) {
	frame := rowsToFrame("B", nil)
	require.Equal(t, "B", frame.RefID)
	require.Equal(t, 0, len(frame.Fields))
}

func TestRowsToFrame_ArraysAndObjectsSerialiseToJSON(t *testing.T) {
	records := []map[string]any{
		{"Tags": []any{"a", "b"}},
	}
	frame := rowsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["a","b"]`, *frame.Fields[0].At(0).(*string))
}

func TestRowsToFrame_MixedTypesFallBackToString(t *testing.T) {
	records := []map[string]any{
		{"val": float64(1)},
		{"val": "text"},
	}
	frame := rowsToFrame("C", records)
	require.Equal(t, 1, len(frame.Fields))
	_, ok := frame.Fields[0].At(0).(*string)
	require.True(t, ok)
}

func TestFlattenRows_MetadataAndProjection(t *testing.T) {
	idx := float64(7)
	items := []rowItem{
		{
			ID: "r1", Name: "Row 1", Index: &idx, Href: "https://h/r1",
			BrowserLink: "https://b/r1", CreatedAt: "2024-01-01T00:00:00.000Z",
			UpdatedAt: "2024-02-01T00:00:00.000Z",
			Values:    map[string]any{"Name": "Alice", "Age": float64(30), "Secret": "x"},
		},
	}

	// No projection: all data columns + all metadata.
	all := flattenRows(items, nil, false)
	require.Len(t, all, 1)
	require.Equal(t, "r1", all[0]["id"])
	require.Equal(t, "Row 1", all[0]["name"])
	require.EqualValues(t, 7, all[0]["index"])
	require.Equal(t, "https://h/r1", all[0]["href"])
	require.Equal(t, "https://b/r1", all[0]["browserLink"])
	require.Equal(t, "Alice", all[0]["Name"])
	require.EqualValues(t, 30, all[0]["Age"])
	require.Equal(t, "x", all[0]["Secret"])

	// Projection keeps only requested data columns; metadata kept when
	// hideSystem is false.
	projected := flattenRows(items, map[string]bool{"Name": true, "Age": true}, false)
	_, hasSecret := projected[0]["Secret"]
	require.False(t, hasSecret)
	require.Equal(t, "Alice", projected[0]["Name"])
	require.Equal(t, "r1", projected[0]["id"])
}

func TestFlattenRows_HideSystemFields(t *testing.T) {
	idx := float64(7)
	items := []rowItem{
		{
			ID: "r1", Name: "Row 1", Index: &idx, Href: "https://h/r1",
			BrowserLink: "https://b/r1", CreatedAt: "2024-01-01T00:00:00.000Z",
			UpdatedAt: "2024-02-01T00:00:00.000Z",
			Values:    map[string]any{"Name": "Alice", "Age": float64(30)},
		},
	}

	rows := flattenRows(items, nil, true)
	require.Len(t, rows, 1)
	// All seven synthetic columns dropped.
	for _, k := range []string{"id", "name", "index", "createdAt", "updatedAt", "href", "browserLink"} {
		_, ok := rows[0][k]
		require.Falsef(t, ok, "system column %q should be hidden", k)
	}
	// User data columns are still present.
	require.Equal(t, "Alice", rows[0]["Name"])
	require.EqualValues(t, 30, rows[0]["Age"])
}

func TestInferColumnType(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("b", []map[string]any{{"b": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00.000Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime(t *testing.T) {
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

	_, ok := toTime("not a date")
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
