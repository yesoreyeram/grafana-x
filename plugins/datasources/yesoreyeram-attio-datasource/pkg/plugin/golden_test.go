package plugin

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

// updateGolden controls whether golden files are (re)written. Regenerate with:
//
//	UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden
//
// Then review and commit the files under pkg/plugin/testdata/.
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

// TestGolden_Frames asserts that the data frames produced for representative
// Attio responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"_record_id": "r1", "name": "Ada", "active": true, "score": float64(100), "close_date": "2024-01-15"},
			{"_record_id": "r2", "name": "Bob", "active": false, "score": float64(0), "close_date": "2024-02-20"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		// Records returned in descending time order; row order must be preserved.
		records := []map[string]any{
			{"_created_at": "2024-03-01T10:00:00.000000000Z", "service": "api", "cpu": float64(80.5), "requests": float64(1200)},
			{"_created_at": "2024-03-01T09:00:00.000000000Z", "service": "api", "cpu": float64(40.0), "requests": float64(600)},
			{"_created_at": "2024-03-01T08:00:00.000000000+05:30", "service": "api", "cpu": float64(12.25), "requests": float64(150)},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_timeseries", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("records_nulls_and_strings", func(t *testing.T) {
		records := []map[string]any{
			{"name": "a", "notes": nil, "mixed": float64(1)},
			{"name": "b", "notes": nil, "mixed": "text"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("records_flattened_values", func(t *testing.T) {
		raw := `[{
			"id":{"record_id":"r1","object_id":"people","workspace_id":"w1"},
			"created_at":"2024-01-02T03:04:05.000000000Z",
			"values":{
				"name":[{"attribute_type":"personal-name","first_name":"Ada","last_name":"Lovelace","full_name":"Ada Lovelace"}],
				"score":[{"attribute_type":"number","value":42}],
				"is_vip":[{"attribute_type":"checkbox","value":true}],
				"amount":[{"attribute_type":"currency","currency_value":99.5,"currency_code":"USD"}],
				"stage":[{"attribute_type":"status","status":{"title":"Won"}}],
				"company":[{"attribute_type":"record-reference","target_object":"companies","target_record_id":"c1"}]
			}
		}]`
		var recs []attioRecord
		if err := json.Unmarshal([]byte(raw), &recs); err != nil {
			t.Fatal(err)
		}
		frame := recordsToFrame("A", flattenRecords(recs, []string{"name", "score", "is_vip", "amount", "stage", "company"}, false))
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_flattened_values", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
