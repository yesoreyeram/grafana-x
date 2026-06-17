package plugin

import (
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

// updateGolden controls whether golden files are (re)written. Regenerate with:
//
//	UPDATE_GOLDEN=true go test ./pkg/plugin/...
//
// Then review and commit the files under pkg/plugin/testdata/.
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

// TestGolden_Frames asserts that the data frames produced for representative
// NocoDB responses match the committed golden files. These lock down the full
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
		// DateTime values with zone offsets, returned descending (as if sorted
		// `-Timestamp`); row order must be preserved.
		records := []map[string]any{
			{"Timestamp": "2024-03-01 10:00:00+00:00", "Service": "api", "CPU": float64(80.5), "Requests": float64(1200)},
			{"Timestamp": "2024-03-01 09:00:00+00:00", "Service": "api", "CPU": float64(40.0), "Requests": float64(600)},
			{"Timestamp": "2024-03-01 08:00:00+05:30", "Service": "api", "CPU": float64(12.25), "Requests": float64(150)},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_timeseries", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("records_nulls_and_strings", func(t *testing.T) {
		// A column that is all-null stays string; a mixed column falls back to string.
		records := []map[string]any{
			{"Name": "a", "Notes": nil, "Mixed": float64(1)},
			{"Name": "b", "Notes": nil, "Mixed": "text"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
