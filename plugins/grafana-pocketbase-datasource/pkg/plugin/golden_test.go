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
// PocketBase responses match the committed golden files. These lock down the
// full data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"id": "r1", "name": "Alice", "active": true, "score": float64(49.5), "joined": "2024-01-15 00:00:00.000Z"},
			{"id": "r2", "name": "Bob", "active": false, "score": float64(0), "joined": "2024-02-20 00:00:00.000Z"},
		}
		frame := recordsToFrame("A", records, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		// DateTime values returned descending (as if sorted by ts desc); row order
		// must be preserved.
		records := []map[string]any{
			{"ts": "2024-03-01 10:00:00.000Z", "service": "api", "cpu": float64(80.5), "requests": float64(1200)},
			{"ts": "2024-03-01 09:00:00.000Z", "service": "api", "cpu": float64(40.0), "requests": float64(600)},
			{"ts": "2024-03-01 08:00:00.000+05:30", "service": "api", "cpu": float64(12.25), "requests": float64(150)},
		}
		frame := recordsToFrame("A", records, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_timeseries", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("records_nulls_and_strings", func(t *testing.T) {
		// A column that is all-null stays string; a mixed column falls back to string.
		records := []map[string]any{
			{"name": "a", "notes": nil, "mixed": float64(1)},
			{"name": "b", "notes": nil, "mixed": "text"},
		}
		frame := recordsToFrame("A", records, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
