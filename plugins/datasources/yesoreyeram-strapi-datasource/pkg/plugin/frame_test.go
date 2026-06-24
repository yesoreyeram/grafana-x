package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypes(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "attributes": map[string]any{"title": "first", "active": true, "views": float64(100)}},
		{"id": float64(2), "attributes": map[string]any{"title": "second", "active": false, "views": float64(200)}},
	}

	frame := recordsToFrame("A", records)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 2, rowLen)
}

func TestRecordsToFrame_FlattenV4DataFormat(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "attributes": map[string]any{"title": "hello"}},
	}
	frame := recordsToFrame("A", records)

	require.Len(t, frame.Fields, 2)
	var fieldNames []string
	for _, f := range frame.Fields {
		fieldNames = append(fieldNames, f.Name)
	}
	require.Contains(t, fieldNames, "id")
	require.Contains(t, fieldNames, "title")
}

func TestRecordsToFrame_FlattenV5DataFormat(t *testing.T) {
	// v5: flat fields plus documentId, no attributes wrapper.
	records := []map[string]any{
		{"id": float64(1), "documentId": "abc123", "title": "hello"},
	}
	frame := recordsToFrame("A", records)

	require.Len(t, frame.Fields, 3)
	var fieldNames []string
	for _, f := range frame.Fields {
		fieldNames = append(fieldNames, f.Name)
	}
	require.Contains(t, fieldNames, "id")
	require.Contains(t, fieldNames, "documentId")
	require.Contains(t, fieldNames, "title")
}

func TestRecordsToFrame_NestedRelationSerialisedToJSON(t *testing.T) {
	// A populated relation (v5 nested object) must serialise to a JSON string.
	records := []map[string]any{
		{"id": float64(1), "title": "a", "author": map[string]any{"id": float64(7), "name": "Kai"}},
	}
	frame := recordsToFrame("A", records)
	authorField, idx := frame.FieldByName("author")
	require.GreaterOrEqual(t, idx, 0)
	require.NotNil(t, authorField)
	require.Equal(t, data.FieldTypeNullableString, authorField.Type())
	s, isStr := authorField.At(0).(*string)
	require.True(t, isStr)
	require.JSONEq(t, `{"id":7,"name":"Kai"}`, *s)
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "attributes": map[string]any{"title": "a", "created_at": "2024-02-03T04:05:06.000Z", "views": float64(1)}},
		{"id": float64(2), "attributes": map[string]any{"title": "b", "created_at": "2024-01-02T03:04:05.000Z", "views": float64(2)}},
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
		{"id": float64(1), "attributes": map[string]any{"title": "b", "created_at": "2024-02-03T04:05:06.000Z"}},
		{"id": float64(2), "attributes": map[string]any{"title": "a", "created_at": "2024-01-02T03:04:05.000Z"}},
	}
	frame := recordsToFrame("A", records)

	// Time field should be first
	require.Equal(t, "created_at", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"id": float64(3), "attributes": map[string]any{"views": float64(40), "title": "c"}},
		{"id": float64(1), "attributes": map[string]any{"views": float64(30), "title": "a"}},
		{"id": float64(2), "attributes": map[string]any{"views": float64(20), "title": "b"}},
	}
	frame := recordsToFrame("A", records)

	viewsField, _ := frame.FieldByName("views")
	require.NotNil(t, viewsField)
	require.EqualValues(t, 40, *viewsField.At(0).(*float64))
	require.EqualValues(t, 30, *viewsField.At(1).(*float64))
	require.EqualValues(t, 20, *viewsField.At(2).(*float64))
}

func TestRecordsToFrame_Empty(t *testing.T) {
	frame := recordsToFrame("B", nil)
	require.Equal(t, "B", frame.RefID)
	require.Equal(t, 0, len(frame.Fields))
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
