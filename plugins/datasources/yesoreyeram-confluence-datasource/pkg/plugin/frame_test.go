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
		{"id": "1", "title": "first", "versionNumber": float64(2), "createdAt": "2024-01-02T03:04:05Z"},
		{"id": "2", "title": "second", "versionNumber": float64(3), "createdAt": "2024-02-03T04:05:06Z"},
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
		{"title": "a", "createdAt": "2024-02-03T04:05:06Z"},
	}
	frame := recordsToFrame("A", records, nil)

	require.NotNil(t, frame.Meta)
	require.Equal(t, data.FrameTypeTable, frame.Meta.Type)
	require.Equal(t, data.FrameTypeVersion{0, 1}, frame.Meta.TypeVersion)

	_, err := frame.RowLen()
	require.NoError(t, err)
}

func TestRecordsToFrame_TimeFieldFirstAsNullableTime(t *testing.T) {
	records := []map[string]any{
		{"title": "b", "createdAt": "2024-02-03T04:05:06Z"},
		{"title": "a", "createdAt": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records, nil)

	require.Equal(t, "createdAt", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"title": "c", "createdAt": "2024-03-04T05:06:07Z"},
		{"title": "b", "createdAt": "2024-02-03T04:05:06Z"},
		{"title": "a", "createdAt": "2024-01-02T03:04:05Z"},
	}
	frame := recordsToFrame("A", records, nil)

	require.Equal(t, "createdAt", frame.Fields[0].Name)
	require.Equal(t, "title", frame.Fields[1].Name)
	require.Equal(t, "c", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "b", *frame.Fields[1].At(1).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(2).(*string))
}

func TestRecordsToFrame_OffsetDateTimeNormalisedToUTC(t *testing.T) {
	records := []map[string]any{
		{"createdAt": "2024-01-01T12:00:00.000+05:30"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2024, 1, 1, 6, 30, 0, 0, time.UTC) // 12:00 +05:30 == 06:30 UTC
	require.True(t, got.Equal(want), "got %v want %v", got, want)
	require.Equal(t, time.UTC, got.Location())
}

func TestRecordsToFrame_NumericStringIDStaysString(t *testing.T) {
	records := []map[string]any{
		{"id": "123456"},
		{"id": "789012"},
	}
	frame := recordsToFrame("A", records, nil)
	require.Equal(t, "id", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[0].Type())
}

func TestRecordsToFrame_FieldSelection(t *testing.T) {
	records := []map[string]any{
		{"id": "1", "title": "a", "status": "current"},
	}
	frame := recordsToFrame("A", records, []string{"id", "title"})
	require.Equal(t, 2, len(frame.Fields))
	names := []string{frame.Fields[0].Name, frame.Fields[1].Name}
	require.ElementsMatch(t, []string{"id", "title"}, names)
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

func TestInferColumnType(t *testing.T) {
	require.Equal(t, fieldTypeNumber, inferColumnType("n", []map[string]any{{"n": float64(1)}}))
	require.Equal(t, fieldTypeBool, inferColumnType("b", []map[string]any{{"b": true}}))
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime_ConfluenceDateFormats(t *testing.T) {
	cases := []string{
		"2024-01-01T00:00:00.000Z",
		"2024-01-01T12:00:00.000+05:30",
		"2024-01-01T00:00:00Z",
		"2024-01-01",
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
}

// ---------------------------------------------------------------------------
// Content flattening
// ---------------------------------------------------------------------------

func itemsFromJSON(t *testing.T, raw string) []json.RawMessage {
	t.Helper()
	var items []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &items))
	return items
}

func TestFlattenContentItems(t *testing.T) {
	items := itemsFromJSON(t, `[{
		"id":"123","status":"current","title":"Release notes","spaceId":"456","authorId":"u1",
		"createdAt":"2024-01-02T03:04:05.000Z",
		"version":{"number":3,"message":"edit","createdAt":"2024-02-02T03:04:05.000Z","authorId":"u2"},
		"_links":{"webui":"/spaces/ENG/pages/123/Release"}
	}]`)

	rows := flattenContentItems(items, "https://acme.atlassian.net")
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, "123", row["id"])
	require.Equal(t, "Release notes", row["title"])
	require.Equal(t, "456", row["spaceId"])
	require.Equal(t, "current", row["status"])
	require.Equal(t, "u1", row["authorId"])
	require.Equal(t, "2024-01-02T03:04:05.000Z", row["createdAt"])
	require.Equal(t, float64(3), row["versionNumber"])
	require.Equal(t, "edit", row["versionMessage"])
	require.Equal(t, "2024-02-02T03:04:05.000Z", row["versionCreatedAt"])
	require.Equal(t, "https://acme.atlassian.net/spaces/ENG/pages/123/Release", row["webui"])
}

func TestFlattenContentItems_MissingVersionIsNil(t *testing.T) {
	items := itemsFromJSON(t, `[{"id":"1","title":"a"}]`)
	rows := flattenContentItems(items, "")
	require.Nil(t, rows[0]["versionNumber"])
	require.Nil(t, rows[0]["versionMessage"])
	require.Nil(t, rows[0]["webui"])
}

func TestFlattenSearchItems(t *testing.T) {
	items := itemsFromJSON(t, `[{
		"content":{"id":"123","type":"page","status":"current","title":"Release notes","spaceId":"456"},
		"title":"Release @@@hl@@@notes@@@endhl@@@",
		"excerpt":"some @@@hl@@@excerpt@@@endhl@@@","url":"/spaces/ENG/pages/123",
		"lastModified":"2024-03-01T10:00:00.000Z"
	}]`)

	rows := flattenSearchItems(items, "https://acme.atlassian.net")
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, "123", row["id"])
	require.Equal(t, "page", row["type"])
	require.Equal(t, "current", row["status"])
	require.Equal(t, "456", row["spaceId"])
	require.Equal(t, "Release notes", row["title"])
	require.Equal(t, "some excerpt", row["excerpt"])
	require.Equal(t, "https://acme.atlassian.net/spaces/ENG/pages/123", row["url"])
	require.Equal(t, "2024-03-01T10:00:00.000Z", row["lastModified"])
}

func TestStripHighlight(t *testing.T) {
	require.Equal(t, "hello world", stripHighlight("@@@hl@@@hello@@@endhl@@@ world"))
}

func TestAbsoluteLink(t *testing.T) {
	require.Nil(t, absoluteLink("https://x", " "))
	require.Equal(t, "https://x/a", absoluteLink("https://x", "/a"))
	require.Equal(t, "https://other/a", absoluteLink("https://x", "https://other/a"))
	require.Equal(t, "/a", absoluteLink("", "/a"))
}

func TestFlattenContentItems_FlowsThroughFrame(t *testing.T) {
	items := itemsFromJSON(t, `[
		{"id":"1","title":"a","createdAt":"2024-03-01T10:00:00.000Z","version":{"number":1}},
		{"id":"2","title":"b","createdAt":"2024-02-01T10:00:00.000Z","version":{"number":4}}
	]`)
	rows := flattenContentItems(items, "https://acme.atlassian.net")
	frame := recordsToFrame("A", rows, []string{"id", "title", "createdAt", "versionNumber"})

	created, _ := frame.FieldByName("createdAt")
	require.NotNil(t, created)
	require.Equal(t, data.FieldTypeNullableTime, created.Type())

	vn, _ := frame.FieldByName("versionNumber")
	require.NotNil(t, vn)
	require.Equal(t, data.FieldTypeNullableFloat64, vn.Type())
}
