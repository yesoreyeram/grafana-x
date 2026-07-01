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
// Teable responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		// A flattened table page including the synthetic identity columns Teable
		// records carry (_id, _createdTime, _lastModifiedTime).
		records := []map[string]any{
			{"_id": "rec1", "_createdTime": "2024-01-15T10:00:00.000Z", "_lastModifiedTime": "2024-01-16T10:00:00.000Z", "name": "Alice", "age": float64(30), "active": true, "score": float64(95.5)},
			{"_id": "rec2", "_createdTime": "2024-01-15T11:00:00.000Z", "_lastModifiedTime": "2024-01-17T10:00:00.000Z", "name": "Bob", "age": float64(25), "active": false, "score": float64(87.0)},
			{"_id": "rec3", "_createdTime": "2024-01-15T12:00:00.000Z", "_lastModifiedTime": "2024-01-18T10:00:00.000Z", "name": "Charlie", "age": float64(35), "active": true, "score": float64(92.3)},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		// DateTime values returned descending (as if sorted by timestamp desc);
		// row order must be preserved.
		records := []map[string]any{
			{"name": "Request A", "value": float64(100), "timestamp": "2024-01-15T12:00:00.000Z"},
			{"name": "Request B", "value": float64(200), "timestamp": "2024-01-15T11:00:00.000Z"},
			{"name": "Request C", "value": float64(150), "timestamp": "2024-01-15T10:00:00.000+05:30"},
		}
		frame := recordsToFrame("D", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_timeseries", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("B", nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("records_nulls_and_strings", func(t *testing.T) {
		// A column that is all-null stays string; a mixed column falls back to string.
		records := []map[string]any{
			{"name": "Alice", "age": nil, "email": "alice@example.com", "mixed": float64(1)},
			{"name": nil, "age": float64(30), "email": nil, "mixed": "text"},
			{"name": "Bob", "age": float64(25), "email": "bob@example.com", "mixed": nil},
		}
		frame := recordsToFrame("C", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("E", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
