package plugin

import (
	"encoding/json"
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

func flattenRaw(t *testing.T, raws ...string) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, len(raws))
	for i, r := range raws {
		out = append(out, flattenIntercomRecord(json.RawMessage(r), i))
	}
	return out
}

// TestGolden_Frames asserts that the data frames produced for representative
// Intercom responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"id": "1", "name": "Alice", "active": true, "spend": float64(49.5), "created_at": "2024-01-15T00:00:00Z"},
			{"id": "2", "name": "Bob", "active": false, "spend": float64(0), "created_at": "2024-02-20T00:00:00Z"},
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_conversations", func(t *testing.T) {
		records := flattenRaw(t,
			`{"type":"conversation","id":"101","state":"open","priority":"priority","created_at":1539897198,"updated_at":1539900000,"source":{"type":"conversation","author":{"type":"user","email":"a@b.com"}}}`,
			`{"type":"conversation","id":"102","state":"closed","priority":"not_priority","created_at":1539800000,"updated_at":1539850000,"source":{"type":"conversation","author":{"type":"lead","email":"c@d.com"}}}`,
		)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_conversations", frame, updateGolden)
	})

	t.Run("records_contacts", func(t *testing.T) {
		records := flattenRaw(t,
			`{"type":"contact","id":"c1","role":"user","email":"a@b.com","name":"Alice","created_at":1394539050,"signed_up_at":1394539050,"last_seen_at":0,"custom_attributes":{"plan":"pro"}}`,
			`{"type":"contact","id":"c2","role":"lead","email":"c@d.com","name":"Bob","created_at":1394600000,"signed_up_at":0,"last_seen_at":1394600000,"custom_attributes":{"plan":"free"}}`,
		)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_contacts", frame, updateGolden)
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

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
