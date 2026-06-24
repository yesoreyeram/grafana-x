package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypesAndOrdersColumns(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "Title": "first", "Active": true, "Age": float64(1)},
		{"id": float64(2), "Title": "second", "Active": false, "Age": float64(2)},
	}

	frame := recordsToFrame("A", records, nil)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 2, rowLen)
}

func TestRecordsToFrame_IsDataPlaneTableCompliant(t *testing.T) {
	records := []map[string]any{
		{"Name": "a", "When": float64(1706932106), "Age": float64(1)},
		{"Name": "b", "When": float64(1704174245), "Age": float64(2)},
	}
	frame := recordsToFrame("A", records, map[string]bool{"When": true})

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_DateEpochSecondsBecomeTime(t *testing.T) {
	// 2024-01-15T00:00:00Z = 1705276800 epoch seconds.
	records := []map[string]any{
		{"Name": "a", "SignedUp": float64(1705276800)},
	}
	frame := recordsToFrame("A", records, map[string]bool{"SignedUp": true})

	signed, _ := frame.FieldByName("SignedUp")
	require.NotNil(t, signed)
	require.Equal(t, data.FieldTypeNullableTime, signed.Type())
	got := signed.At(0).(*time.Time)
	require.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), got.UTC())
	require.Equal(t, time.UTC, got.Location())
}

func TestRecordsToFrame_TimeFieldFirst(t *testing.T) {
	records := []map[string]any{
		{"Name": "b", "When": float64(1706932106)},
		{"Name": "a", "When": float64(1704174245)},
	}
	frame := recordsToFrame("A", records, map[string]bool{"When": true})

	require.Equal(t, "When", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
}

func TestRecordsToFrame_DateNullPreserved(t *testing.T) {
	records := []map[string]any{
		{"Name": "a", "When": float64(1705276800)},
		{"Name": "b", "When": nil},
	}
	frame := recordsToFrame("A", records, map[string]bool{"When": true})
	when, _ := frame.FieldByName("When")
	require.NotNil(t, when)
	require.NotNil(t, when.At(0).(*time.Time))
	require.Nil(t, when.At(1).(*time.Time))
}

func TestRecordsToFrame_NoDateMetadataKeepsNumbers(t *testing.T) {
	// Without metadata, epoch numbers stay numeric (no string sniffing).
	records := []map[string]any{
		{"When": float64(1705276800)},
	}
	frame := recordsToFrame("A", records, nil)
	when, _ := frame.FieldByName("When")
	require.NotNil(t, when)
	require.Equal(t, data.FieldTypeNullableFloat64, when.Type())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"Age": float64(40), "Name": "c"},
		{"Age": float64(30), "Name": "a"},
		{"Age": float64(20), "Name": "b"},
	}
	frame := recordsToFrame("A", records, nil)

	ageField, _ := frame.FieldByName("Age")
	require.NotNil(t, ageField)
	require.EqualValues(t, 40, *ageField.At(0).(*float64))
	require.EqualValues(t, 30, *ageField.At(1).(*float64))
	require.EqualValues(t, 20, *ageField.At(2).(*float64))
}

func TestRecordsToFrame_ArraysAndObjectsSerialiseToJSON(t *testing.T) {
	records := []map[string]any{
		{"Tags": []any{"L", "a", "b"}},
	}
	frame := recordsToFrame("A", records, nil)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
	require.Equal(t, `["L","a","b"]`, *frame.Fields[0].At(0).(*string))
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
	require.Equal(t, 1, frame.Fields[0].Len())

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeNumericWide, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)
	require.Equal(t, data.FieldTypeInt64, frame.Fields[0].Type())

	rowLen, err := frame.RowLen()
	require.NoError(t, err)
	require.Equal(t, 1, rowLen)
}
