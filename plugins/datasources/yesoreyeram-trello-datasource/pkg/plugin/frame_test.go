package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCardsToFrame_Basic(t *testing.T) {
	records := []map[string]any{
		{"id": "c1", "name": "Task 1", "closed": false, "pos": float64(1)},
		{"id": "c2", "name": "Task 2", "closed": true, "pos": float64(2)},
	}
	frame := cardsToFrame("A", records)
	require.NotNil(t, frame)
	require.Len(t, frame.Fields, 4)
	require.Equal(t, "A", frame.RefID)
}

func TestCardsToFrame_Empty(t *testing.T) {
	frame := cardsToFrame("A", nil)
	require.Empty(t, frame.Fields)
}

func TestCardsToFrame_TimeColumnsFirst(t *testing.T) {
	records := []map[string]any{
		{"name": "b", "due": "2024-02-01T00:00:00Z"},
		{"name": "a", "due": "2024-01-01T00:00:00Z"},
	}
	frame := cardsToFrame("A", records)
	require.Len(t, frame.Fields, 2)
	require.Equal(t, "due", frame.Fields[0].Name)
	require.Equal(t, "name", frame.Fields[1].Name)
	// Row order preserved.
	require.Equal(t, "b", *frame.Fields[1].At(0).(*string))
	require.Equal(t, "a", *frame.Fields[1].At(1).(*string))
}

func TestFlattenCard_Labels(t *testing.T) {
	card := map[string]any{
		"id":     "c1",
		"name":   "Bug",
		"labels": []any{map[string]any{"id": "l1", "name": "bug", "color": "red"}, map[string]any{"id": "l2", "name": "", "color": "green"}},
	}
	flat := flattenCard(card)
	require.Equal(t, "bug, green", flat["labels"])
}

func TestFlattenCard_LabelsEmpty(t *testing.T) {
	card := map[string]any{
		"id":     "c1",
		"labels": []any{},
	}
	flat := flattenCard(card)
	require.Nil(t, flat["labels"])
}

func TestFlattenCard_StringArray(t *testing.T) {
	card := map[string]any{
		"idMembers": []any{"u1", "u2"},
	}
	flat := flattenCard(card)
	require.Equal(t, "u1, u2", flat["idMembers"])
}

func TestFlattenCard_StringArrayNil(t *testing.T) {
	card := map[string]any{}
	flat := flattenCard(card)
	require.Nil(t, flat["idMembers"])
}

func TestInferColumnType_TrelloDates(t *testing.T) {
	records := []map[string]any{
		{"due": "2024-01-01T00:00:00Z"},
		{"due": "2024-02-01T12:30:00Z"},
	}
	require.Equal(t, fieldTypeTime, inferColumnType("due", records))

	records = []map[string]any{{"pos": float64(3)}, {"pos": float64(5)}}
	require.Equal(t, fieldTypeNumber, inferColumnType("pos", records))

	records = []map[string]any{{"closed": true}, {"closed": false}}
	require.Equal(t, fieldTypeBool, inferColumnType("closed", records))
}

func TestToColumnTime_RFC3339(t *testing.T) {
	tm, ok := toColumnTime("due", "2024-01-01T00:00:00Z")
	require.True(t, ok)
	require.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), tm.UTC())

	_, ok = toColumnTime("name", "2024-01-01T00:00:00Z")
	require.False(t, ok)
}

func TestCountToFrame(t *testing.T) {
	frame := countToFrame("A", 42)
	require.Len(t, frame.Fields, 1)
	require.Equal(t, "count", frame.Fields[0].Name)
	require.Equal(t, int64(42), frame.Fields[0].At(0))
}

func TestFlattenBadges(t *testing.T) {
	badges := map[string]any{
		"attachments":       float64(3),
		"comments":          float64(5),
		"votes":             float64(1),
		"checkItems":        float64(4),
		"checkItemsChecked": float64(2),
		"subscribed":        true, // non-count fields are ignored
		"due":               nil,
	}
	flat := flattenBadges(badges)
	require.Equal(t, float64(3), flat["badges_attachments"])
	require.Equal(t, float64(5), flat["badges_comments"])
	require.Equal(t, float64(1), flat["badges_votes"])
	require.Equal(t, float64(4), flat["badges_checkItems"])
	require.Equal(t, float64(2), flat["badges_checkItemsChecked"])
	_, hasSubscribed := flat["badges_subscribed"]
	require.False(t, hasSubscribed)
}

func TestFlattenCard_BadgesExpandedToColumns(t *testing.T) {
	card := map[string]any{
		"id":     "c1",
		"badges": map[string]any{"votes": float64(2), "comments": float64(7)},
	}
	flat := flattenCard(card)
	require.Equal(t, float64(2), flat["badges_votes"])
	require.Equal(t, float64(7), flat["badges_comments"])
	_, hasBadges := flat["badges"]
	require.False(t, hasBadges) // raw badges object is replaced by columns
}

func TestFlattenCard_DateCreatedFromID(t *testing.T) {
	// ObjectId with timestamp prefix 0x5e8f1a2b = 1586436651 (2020-04-09).
	card := map[string]any{"id": "5e8f1a2b3c4d5e6f7a8b9c0d", "name": "Card"}
	flat := flattenCard(card)
	require.Equal(t, "2020-04-09T12:50:51Z", flat["dateCreated"])

	// Short / non-ObjectId ids do not get a derived dateCreated.
	flat = flattenCard(map[string]any{"id": "c1"})
	_, ok := flat["dateCreated"]
	require.False(t, ok)
}

func TestCardCreatedUnix(t *testing.T) {
	sec, ok := cardCreatedUnix("5e8f1a2b3c4d5e6f7a8b9c0d")
	require.True(t, ok)
	require.EqualValues(t, 1586436651, sec)

	_, ok = cardCreatedUnix("short")
	require.False(t, ok)

	_, ok = cardCreatedUnix("zzzzzzzz3c4d5e6f7a8b9c0d")
	require.False(t, ok)
}

func TestEarliestCardID(t *testing.T) {
	cards := []map[string]any{
		{"id": "5e8f1a2b000000000000000a"}, // ts 0x5e8f1a2b
		{"id": "5e8f1a29000000000000000b"}, // ts 0x5e8f1a29 (oldest)
		{"id": "5e8f1a2c000000000000000c"}, // ts 0x5e8f1a2c
	}
	require.Equal(t, "5e8f1a29000000000000000b", earliestCardID(cards))
	require.Equal(t, "", earliestCardID(nil))
}

func TestToTrelloDate(t *testing.T) {
	require.Equal(t, "", toTrelloDate(""))
	require.Equal(t, "2024-01-01T00:00:00Z", toTrelloDate("2024-01-01"))
	// Unix millis.
	require.Equal(t, "2025-01-01T00:00:00Z", toTrelloDate("1735689600000"))
	// Unix seconds.
	require.Equal(t, "2025-01-01T00:00:00Z", toTrelloDate("1735689600"))
	// Unknown / card id passes through unchanged.
	require.Equal(t, "5e8f1a2b3c4d5e6f7a8b9c0d", toTrelloDate("5e8f1a2b3c4d5e6f7a8b9c0d"))
}

func TestSelectCardFields(t *testing.T) {
	record := map[string]any{"id": "c1", "name": "Task", "closed": false}
	result := selectCardFields(record, []string{"name"})
	require.Len(t, result, 1)
	require.Equal(t, "Task", result["name"])
}

func TestOrderedColumns(t *testing.T) {
	records := []map[string]any{
		{"name": "a", "id": "1"},
		{"name": "b", "id": "2", "extra": "x"},
	}
	cols := orderedColumns(records)
	require.Contains(t, cols, "id")
	require.Contains(t, cols, "name")
	require.Contains(t, cols, "extra")
}
