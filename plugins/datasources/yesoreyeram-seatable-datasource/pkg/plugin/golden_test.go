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
// SeaTable responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering (identity + time columns first), row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"_id": "r1", "_ctime": "2024-01-15T10:00:00.000+00:00", "_mtime": "2024-01-16T08:30:00.000+00:00", "name": "Alice", "age": float64(30), "active": true, "score": float64(95.5)},
			{"_id": "r2", "_ctime": "2024-02-20T11:00:00.000+00:00", "_mtime": "2024-02-21T09:15:00.000+00:00", "name": "Bob", "age": float64(25), "active": false, "score": float64(87)},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		// Values returned descending (as if sorted by timestamp desc); row order
		// must be preserved.
		records := []map[string]any{
			{"timestamp": "2024-03-01T10:00:00.000+00:00", "service": "api", "cpu": float64(80.5), "requests": float64(1200)},
			{"timestamp": "2024-03-01T09:00:00.000+00:00", "service": "api", "cpu": float64(40), "requests": float64(600)},
			{"timestamp": "2024-03-01T08:00:00.000+05:30", "service": "api", "cpu": float64(12.25), "requests": float64(150)},
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
			{"name": "a", "notes": nil, "mixed": float64(1)},
			{"name": "b", "notes": nil, "mixed": "text"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
