package plugin

import (
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		// SignedUp is a Grist Date column: epoch seconds, converted via metadata.
		records := []map[string]any{
			{"id": float64(1), "Name": "Alice", "Active": true, "MRR": float64(49.5), "SignedUp": float64(1705276800)},
			{"id": float64(2), "Name": "Bob", "Active": false, "MRR": float64(0), "SignedUp": float64(1708387200)},
		}
		frame := recordsToFrame("A", records, map[string]bool{"SignedUp": true})
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_date_epoch", func(t *testing.T) {
		// Day is a Date column (midnight UTC); Moment is a DateTime column.
		records := []map[string]any{
			{"id": float64(1), "Day": float64(1705276800), "Moment": float64(1705310645)},
			{"id": float64(2), "Day": float64(1705363200), "Moment": nil},
		}
		frame := recordsToFrame("A", records, map[string]bool{"Day": true, "Moment": true})
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_date_epoch", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		records := []map[string]any{
			{"Timestamp": float64(1709287200), "Service": "api", "CPU": float64(80.5), "Requests": float64(1200)},
			{"Timestamp": float64(1709283600), "Service": "api", "CPU": float64(40.0), "Requests": float64(600)},
			{"Timestamp": float64(1709280000), "Service": "api", "CPU": float64(12.25), "Requests": float64(150)},
		}
		frame := recordsToFrame("A", records, map[string]bool{"Timestamp": true})
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_timeseries", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("records_nulls_and_strings", func(t *testing.T) {
		records := []map[string]any{
			{"Name": "a", "Notes": nil, "Mixed": float64(1)},
			{"Name": "b", "Notes": nil, "Mixed": "text"},
		}
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
