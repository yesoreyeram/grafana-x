package plugin

import (
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

func TestGolden_Frames(t *testing.T) {
	// records_mixed_types exercises the Strapi v4 response shape (fields nested
	// under attributes).
	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"id": float64(1), "attributes": map[string]any{"title": "Hello", "published": true, "views": float64(100), "created_at": "2024-01-15"}},
			{"id": float64(2), "attributes": map[string]any{"title": "World", "published": false, "views": float64(0), "created_at": "2024-02-20"}},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	// records_v5_flat exercises the Strapi v5 response shape (flat fields plus a
	// documentId, no attributes wrapper).
	t.Run("records_v5_flat", func(t *testing.T) {
		records := []map[string]any{
			{"id": float64(1), "documentId": "abc123", "title": "Hello", "published": true, "views": float64(100), "created_at": "2024-01-15"},
			{"id": float64(2), "documentId": "def456", "title": "World", "published": false, "views": float64(0), "created_at": "2024-02-20"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_v5_flat", frame, updateGolden)
	})

	t.Run("records_timeseries", func(t *testing.T) {
		records := []map[string]any{
			{"id": float64(1), "attributes": map[string]any{"timestamp": "2024-03-01T10:00:00.000Z", "service": "api", "cpu": float64(80.5), "requests": float64(1200)}},
			{"id": float64(2), "attributes": map[string]any{"timestamp": "2024-03-01T09:00:00.000Z", "service": "api", "cpu": float64(40.0), "requests": float64(600)}},
			{"id": float64(3), "attributes": map[string]any{"timestamp": "2024-03-01T08:00:00.000+05:30", "service": "api", "cpu": float64(12.25), "requests": float64(150)}},
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
			{"id": float64(1), "attributes": map[string]any{"title": "a", "notes": nil, "mixed": float64(1)}},
			{"id": float64(2), "attributes": map[string]any{"title": "b", "notes": nil, "mixed": "text"}},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
