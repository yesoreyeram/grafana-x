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
// Coda responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("rows_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"id": "row1", "Name": "Alice", "Active": true, "MRR": float64(49.5), "SignedUp": "2024-01-15"},
			{"id": "row2", "Name": "Bob", "Active": false, "MRR": float64(0), "SignedUp": "2024-02-20"},
		}
		frame := rowsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "rows_mixed_types", frame, updateGolden)
	})

	t.Run("rows_timeseries", func(t *testing.T) {
		// Date values returned descending; row order must be preserved.
		records := []map[string]any{
			{"Timestamp": "2024-03-01T10:00:00.000Z", "Service": "api", "CPU": float64(80.5)},
			{"Timestamp": "2024-03-01T09:00:00.000Z", "Service": "api", "CPU": float64(40.0)},
			{"Timestamp": "2024-03-01T08:00:00.000+05:30", "Service": "api", "CPU": float64(12.25)},
		}
		frame := rowsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "rows_timeseries", frame, updateGolden)
	})

	t.Run("rows_empty", func(t *testing.T) {
		frame := rowsToFrame("A", nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "rows_empty", frame, updateGolden)
	})

	t.Run("rows_flattened", func(t *testing.T) {
		// Exercises the full flatten path: a Coda `values` map plus row metadata
		// (createdAt/updatedAt time fields, id/name/index/href/browserLink), with
		// an array cell value serialised to JSON.
		idx := float64(7)
		items := []rowItem{
			{
				ID: "r1", Name: "Alice", Index: &idx, Href: "https://coda.io/apis/v1/r1",
				BrowserLink: "https://coda.io/d/_dX#_rr1",
				CreatedAt:   "2024-01-02T03:04:05.000Z",
				UpdatedAt:   "2024-02-03T04:05:06.000Z",
				Values:      map[string]any{"Name": "Alice", "Tags": []any{"a", "b"}, "Score": float64(95)},
			},
			{
				ID: "r2", Name: "Bob", Href: "https://coda.io/apis/v1/r2",
				BrowserLink: "https://coda.io/d/_dX#_rr2",
				CreatedAt:   "2024-01-03T03:04:05.000Z",
				UpdatedAt:   "2024-02-04T04:05:06.000Z",
				Values:      map[string]any{"Name": "Bob", "Tags": nil, "Score": float64(87)},
			},
		}
		frame := rowsToFrame("A", flattenRows(items, nil, false))
		experimental.CheckGoldenJSONFrame(t, goldenDir, "rows_flattened", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
