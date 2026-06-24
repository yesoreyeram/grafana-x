package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

func TestRecordsToFrame_InfersTypes(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "name": "Ada", "active": true, "score": float64(100)},
		{"id": float64(2), "name": "Bob", "active": false, "score": float64(200)},
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
		{"name": "a", "_created_at": "2024-02-03T04:05:06.000Z", "score": float64(1)},
		{"name": "b", "_created_at": "2024-01-02T03:04:05.000Z", "score": float64(2)},
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
		{"name": "b", "_created_at": "2024-02-03T04:05:06.000Z"},
		{"name": "a", "_created_at": "2024-01-02T03:04:05.000Z"},
	}
	frame := recordsToFrame("A", records)

	require.Equal(t, "_created_at", frame.Fields[0].Name)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, time.UTC, frame.Fields[0].At(0).(*time.Time).Location())
}

func TestRecordsToFrame_PreservesRowOrder(t *testing.T) {
	records := []map[string]any{
		{"score": float64(40), "name": "c"},
		{"score": float64(30), "name": "a"},
		{"score": float64(20), "name": "b"},
	}
	frame := recordsToFrame("A", records)

	scoreField, _ := frame.FieldByName("score")
	require.NotNil(t, scoreField)
	require.EqualValues(t, 40, *scoreField.At(0).(*float64))
	require.EqualValues(t, 30, *scoreField.At(1).(*float64))
	require.EqualValues(t, 20, *scoreField.At(2).(*float64))
}

func TestRecordsToFrame_DateStringParsedToTime(t *testing.T) {
	records := []map[string]any{
		{"close_date": "2023-01-15"},
	}
	frame := recordsToFrame("A", records)
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	got := frame.Fields[0].At(0).(*time.Time)
	want := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	require.True(t, got.Equal(want), "got %v want %v", got, want)
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
	require.Equal(t, fieldTypeTime, inferColumnType("t", []map[string]any{{"t": "2024-01-01T00:00:00.000Z"}}))
	require.Equal(t, fieldTypeString, inferColumnType("s", []map[string]any{{"s": "x"}}))
	require.Equal(t, fieldTypeString, inferColumnType("missing", []map[string]any{{"other": 1}}))
}

func TestToTime_AttioFormats(t *testing.T) {
	cases := []string{
		"2026-06-15T09:30:55.000000000Z",
		"2026-06-15T09:30:55.000Z",
		"2026-06-15T09:30:55.000+05:30",
		"2024-01-01T00:00:00Z",
		"2024-01-01",
	}
	for _, c := range cases {
		_, ok := toTime(c)
		require.True(t, ok, "expected %q to parse as time", c)
	}
	_, ok := toTime("not a time")
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

// --- flattening ----------------------------------------------------------

func decodeRecords(t *testing.T, raw string) []attioRecord {
	t.Helper()
	var recs []attioRecord
	require.NoError(t, json.Unmarshal([]byte(raw), &recs))
	return recs
}

func TestFlattenRecords_ValueTypes(t *testing.T) {
	raw := `[{
		"id":{"record_id":"r1","object_id":"o1","workspace_id":"w1"},
		"created_at":"2024-01-02T03:04:05.000000000Z",
		"values":{
			"name":[{"attribute_type":"personal-name","first_name":"Ada","last_name":"Lovelace","full_name":"Ada Lovelace"}],
			"description":[{"attribute_type":"text","value":"A mathematician"}],
			"score":[{"attribute_type":"number","value":42}],
			"is_vip":[{"attribute_type":"checkbox","value":true}],
			"close_date":[{"attribute_type":"date","value":"2023-01-15"}],
			"amount":[{"attribute_type":"currency","currency_value":99.5,"currency_code":"USD"}],
			"stage":[{"attribute_type":"status","status":{"title":"Won"}}],
			"category":[{"attribute_type":"select","option":{"title":"Enterprise"}}],
			"company":[{"attribute_type":"record-reference","target_object":"companies","target_record_id":"c1"}],
			"owner":[{"attribute_type":"actor-reference","referenced_actor_type":"workspace-member","referenced_actor_id":"m1"}],
			"email_addresses":[{"attribute_type":"email-address","email_address":"ada@attio.com"}],
			"primary_domain":[{"attribute_type":"domain","domain":"attio.com","root_domain":"attio.com"}]
		}
	}]`
	records := flattenRecords(decodeRecords(t, raw), nil)
	require.Len(t, records, 1)
	row := records[0]

	require.Equal(t, "r1", row["_record_id"])
	require.Equal(t, "2024-01-02T03:04:05.000000000Z", row["_created_at"])
	require.Equal(t, "Ada Lovelace", row["name"])
	require.Equal(t, "A mathematician", row["description"])
	require.EqualValues(t, 42, row["score"])
	require.Equal(t, true, row["is_vip"])
	require.Equal(t, "2023-01-15", row["close_date"])
	require.EqualValues(t, 99.5, row["amount"])
	require.Equal(t, "Won", row["stage"])
	require.Equal(t, "Enterprise", row["category"])
	require.Equal(t, "c1", row["company"])
	require.Equal(t, "m1", row["owner"])
	require.Equal(t, "ada@attio.com", row["email_addresses"])
	require.Equal(t, "attio.com", row["primary_domain"])
}

func TestFlattenRecords_SelectStatusAsBareUUID(t *testing.T) {
	raw := `[{
		"id":{"record_id":"r1"},
		"values":{
			"stage":[{"attribute_type":"status","status":"11f07f01-c10f-4e05-a522-33e050bc52ee"}],
			"category":[{"attribute_type":"select","option":"08c2c59a-c18e-40c6-8dc4-95415313b2ea"}]
		}
	}]`
	records := flattenRecords(decodeRecords(t, raw), nil)
	require.Equal(t, "11f07f01-c10f-4e05-a522-33e050bc52ee", records[0]["stage"])
	require.Equal(t, "08c2c59a-c18e-40c6-8dc4-95415313b2ea", records[0]["category"])
}

func TestFlattenRecords_EmptyValueArrayIsNil(t *testing.T) {
	raw := `[{"id":{"record_id":"r1"},"values":{"notes":[]}}]`
	records := flattenRecords(decodeRecords(t, raw), nil)
	require.Nil(t, records[0]["notes"])
}

func TestFlattenRecords_TakesFirstActiveValue(t *testing.T) {
	raw := `[{"id":{"record_id":"r1"},"values":{"score":[{"attribute_type":"number","value":10},{"attribute_type":"number","value":99}]}}]`
	records := flattenRecords(decodeRecords(t, raw), nil)
	require.EqualValues(t, 10, records[0]["score"])
}

func TestFlattenRecords_FieldProjection(t *testing.T) {
	raw := `[{
		"id":{"record_id":"r1"},
		"values":{
			"name":[{"attribute_type":"text","value":"Ada"}],
			"score":[{"attribute_type":"number","value":1}]
		}
	}]`
	records := flattenRecords(decodeRecords(t, raw), []string{"name"})
	require.Equal(t, "Ada", records[0]["name"])
	_, hasScore := records[0]["score"]
	require.False(t, hasScore)
	// identity column always retained
	require.Equal(t, "r1", records[0]["_record_id"])
}

func TestFlattenRecords_UnknownTypeSerialisedToJSON(t *testing.T) {
	raw := `[{"id":{"record_id":"r1"},"values":{"hq":[{"attribute_type":"location","locality":"Cupertino","region":"CA"}]}}]`
	records := flattenRecords(decodeRecords(t, raw), nil)
	s, ok := records[0]["hq"].(string)
	require.True(t, ok)
	require.Contains(t, s, "Cupertino")
}
