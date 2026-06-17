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
// Appwrite responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("documents_mixed_types", func(t *testing.T) {
		documents := []map[string]any{
			{"$id": "d1", "name": "Alice", "active": true, "mrr": float64(49.5), "signedUp": "2024-01-15"},
			{"$id": "d2", "name": "Bob", "active": false, "mrr": float64(0), "signedUp": "2024-02-20"},
		}
		frame := documentsToFrame("A", documents, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "documents_mixed_types", frame, updateGolden)
	})

	t.Run("documents_timeseries", func(t *testing.T) {
		// DateTime values returned descending (as if sorted by ts desc); row order
		// must be preserved.
		documents := []map[string]any{
			{"ts": "2024-03-01T10:00:00.000+00:00", "service": "api", "cpu": float64(80.5), "requests": float64(1200)},
			{"ts": "2024-03-01T09:00:00.000+00:00", "service": "api", "cpu": float64(40.0), "requests": float64(600)},
			{"ts": "2024-03-01T08:00:00.000+05:30", "service": "api", "cpu": float64(12.25), "requests": float64(150)},
		}
		frame := documentsToFrame("A", documents, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "documents_timeseries", frame, updateGolden)
	})

	t.Run("documents_empty", func(t *testing.T) {
		frame := documentsToFrame("A", nil, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "documents_empty", frame, updateGolden)
	})

	t.Run("documents_nulls_and_strings", func(t *testing.T) {
		// A column that is all-null stays string; a mixed column falls back to string.
		documents := []map[string]any{
			{"name": "a", "notes": nil, "mixed": float64(1)},
			{"name": "b", "notes": nil, "mixed": "text"},
		}
		frame := documentsToFrame("A", documents, false)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "documents_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
