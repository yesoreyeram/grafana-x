package plugin

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

func decodePages(raw string, out *[]notionPage) {
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		panic(err)
	}
}

// updateGolden controls whether golden files are (re)written. Regenerate with:
//
//	UPDATE_GOLDEN=true go test ./pkg/plugin/...
//
// Then review and commit the files under pkg/plugin/testdata/.
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

// TestGolden_Frames asserts that the data frames produced for representative
// Notion responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"Id": float64(1), "Name": "Alice", "Active": true, "MRR": float64(49.5), "SignedUp": "2024-01-15"},
			{"Id": float64(2), "Name": "Bob", "Active": false, "MRR": float64(0), "SignedUp": "2024-02-20"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		// Date values returned descending (as if sorted `-Timestamp`); row order
		// must be preserved.
		records := []map[string]any{
			{"Timestamp": "2024-03-01T10:00:00.000Z", "Service": "api", "CPU": float64(80.5), "Requests": float64(1200)},
			{"Timestamp": "2024-03-01T09:00:00.000Z", "Service": "api", "CPU": float64(40.0), "Requests": float64(600)},
			{"Timestamp": "2024-03-01T08:00:00.000+05:30", "Service": "api", "CPU": float64(12.25), "Requests": float64(150)},
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
			{"Name": "a", "Notes": nil, "Mixed": float64(1)},
			{"Name": "b", "Notes": nil, "Mixed": "text"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("records_flattened_pages", func(t *testing.T) {
		pages := []notionPage{}
		raw := `[{
			"id":"p1",
			"properties":{
				"Name":{"type":"title","title":[{"plain_text":"Alice"}]},
				"Active":{"type":"checkbox","checkbox":true},
				"MRR":{"type":"number","number":49.5},
				"Tags":{"type":"multi_select","multi_select":[{"name":"vip"},{"name":"beta"}]}
			}
		}]`
		decodePages(raw, &pages)
		frame := recordsToFrame("A", flattenPages(pages, []string{"Name", "Active", "MRR", "Tags"}))
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_flattened_pages", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
