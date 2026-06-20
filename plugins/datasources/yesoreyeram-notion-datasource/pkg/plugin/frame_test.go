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
		{"Id": float64(1), "Title": "first", "Active": true, "CreatedAt": "2024-01-02T03:04:05Z"},
		{"Id": float64(2), "Title": "second", "Active": false, "CreatedAt": "2024-02-03T04:05:06Z"},
	}

	frame := recordsToFrame("A", records)
	require.Equal(t, "A", frame.RefID)
	require.Equal(t, 4, len(frame.Fields))
	require.Equal(t, 2, frame.Rows())

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

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

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

	require.Equal(t, "CreatedAt", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	// The frame layer must NOT re-sort rows: the order returned by Notion (which
	// honours the query's sorts) must be preserved exactly.
	records := []map[string]any{
		{"Name": "c", "CreatedAt": "2024-03-04T05:06:07Z"},
		{"Name": "b", "CreatedAt": "2024-02-03T04:05:06Z"},
		{"Name": "a", "CreatedAt": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "CreatedAt", frame.Fields[0].Name)
	require.Equal(t, "Name", frame.Fields[1].Name)

	require.Equal(t, "c", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "b", *frame.Fields[1].At(1).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(2).(*string))
}

func TestRecordsToFrame_OffsetDateTimeNormalisedToUTC(t *testing.T) {
	records := []map[string]any{
		{"ts": "2024-01-01T12:00:00.000+05:30"},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC) // 12:00 +05:30 == 06:30 UTC
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
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
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime_NotionDateFormats(t *testing.T) {
	cases := []string{
		"2024-01-01T00:00:00.000Z",      // Notion timestamp with millis
		"2024-01-01T12:00:00.000+05:30", // offset zone with millis
		"2024-01-01T00:00:00Z",          // RFC3339
		"2024-01-01",                    // date only
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}

	_, ok := toTime("just some text")
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

// ---------------------------------------------------------------------------
// Page flattening
// ---------------------------------------------------------------------------

func pagesFromJSON(t *testing.T, raw string) []notionPage {
	t.Helper()
	var pages []notionPage
	require.NoError(t, json.Unmarshal([]byte(raw), &pages))
	return pages
}

func TestFlattenPages_AllPropertyTypes(t *testing.T) {
	pages := pagesFromJSON(t, `[{
		"id": "page-1",
		"created_time": "2024-01-02T03:04:05.000Z",
		"last_edited_time": "2024-02-02T03:04:05.000Z",
		"properties": {
			"Name":   {"type":"title","title":[{"plain_text":"Al"},{"plain_text":"ice"}]},
			"Notes":  {"type":"rich_text","rich_text":[{"plain_text":"hello"}]},
			"MRR":    {"type":"number","number":49.5},
			"Active": {"type":"checkbox","checkbox":true},
			"Stage":  {"type":"select","select":{"name":"Pro"}},
			"State":  {"type":"status","status":{"name":"Open"}},
			"Tags":   {"type":"multi_select","multi_select":[{"name":"a"},{"name":"b"}]},
			"Due":    {"type":"date","date":{"start":"2024-03-01"}},
			"Owner":  {"type":"people","people":[{"name":"Sam"}]},
			"Email":  {"type":"email","email":"a@b.com"},
			"Empty":  {"type":"rich_text","rich_text":[]}
		}
	}]`)

	rows := flattenPages(pages, nil)
	require.Len(t, rows, 1)
	row := rows[0]

	require.Equal(t, "Alice", row["Name"])
	require.Equal(t, "hello", row["Notes"])
	require.Equal(t, 49.5, row["MRR"])
	require.Equal(t, true, row["Active"])
	require.Equal(t, "Pro", row["Stage"])
	require.Equal(t, "Open", row["State"])
	require.Equal(t, "a, b", row["Tags"])
	require.Equal(t, "2024-03-01", row["Due"])
	require.Equal(t, "Sam", row["Owner"])
	require.Equal(t, "a@b.com", row["Email"])
	require.Nil(t, row["Empty"]) // empty rich text becomes null
	// Page metadata columns are always present.
	require.Equal(t, "page-1", row["id"])
	require.Equal(t, "2024-01-02T03:04:05.000Z", row["created_time"])
	require.Equal(t, "2024-02-02T03:04:05.000Z", row["last_edited_time"])
}

func TestFlattenPages_FieldsFilterKeepsMetadata(t *testing.T) {
	pages := pagesFromJSON(t, `[{
		"id": "page-1",
		"created_time": "2024-01-02T03:04:05.000Z",
		"properties": {
			"Name": {"type":"title","title":[{"plain_text":"Alice"}]},
			"MRR":  {"type":"number","number":10}
		}
	}]`)

	rows := flattenPages(pages, []string{"Name"})
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, "Alice", row["Name"])
	_, hasMRR := row["MRR"]
	require.False(t, hasMRR, "MRR should be filtered out")
	// Metadata is always retained.
	require.Equal(t, "page-1", row["id"])
}

func TestFlattenProperty_Formula(t *testing.T) {
	raw := json.RawMessage(`{"type":"formula","formula":{"type":"number","number":7}}`)
	require.Equal(t, float64(7), flattenProperty(raw))

	raw = json.RawMessage(`{"type":"formula","formula":{"type":"string","string":"x"}}`)
	require.Equal(t, "x", flattenProperty(raw))
}

func TestFlattenPages_FlowsThroughFrame(t *testing.T) {
	pages := pagesFromJSON(t, `[{
		"id":"p1",
		"properties":{
			"Name":{"type":"title","title":[{"plain_text":"Alice"}]},
			"MRR":{"type":"number","number":49.5}
		}
	},{
		"id":"p2",
		"properties":{
			"Name":{"type":"title","title":[{"plain_text":"Bob"}]},
			"MRR":{"type":"number","number":0}
		}
	}]`)
	rows := flattenPages(pages, []string{"Name", "MRR"})
	frame := recordsToFrame("A", rows)

	mrr, _ := frame.FieldByName("MRR")
	require.NotNil(t, mrr)
	require.Equal(t, data.FieldTypeNullableFloat64, mrr.Type())
	require.EqualValues(t, 49.5, *mrr.At(0).(*float64))
}
