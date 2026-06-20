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

func flattenRaw(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatal(err)
	}
	records := make([]map[string]any, 0, len(items))
	for _, n := range items {
		records = append(records, flattenEntity(n))
	}
	return records
}

// TestGolden_Frames asserts that the data frames produced for representative
// Plane responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("workitems_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"id":"9hx","name":"Fix login bug","priority":"high","sequence_id":12,
			 "state":{"id":"st1","name":"In Progress","group":"started"},
			 "assignees":[{"id":"u1","display_name":"Alice"},{"id":"u2","display_name":"Bob"}],
			 "labels":[{"id":"l1","name":"bug"}],
			 "created_by":{"id":"u9","display_name":"Carol"},
			 "project":{"id":"p1","name":"Apollo"},
			 "is_draft":false,
			 "created_at":"2024-01-01T09:00:00Z","updated_at":"2024-01-02T10:30:00Z","target_date":"2024-02-01"},
			{"id":"9hz","name":"Update docs","priority":"low","sequence_id":13,
			 "state":{"id":"st2","name":"Todo","group":"unstarted"},
			 "assignees":[],
			 "labels":[],
			 "created_by":{"id":"u9","display_name":"Carol"},
			 "project":{"id":"p1","name":"Apollo"},
			 "is_draft":false,
			 "created_at":"2024-01-01T09:05:00Z","updated_at":"2024-01-01T09:05:00Z","target_date":null}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "workitems_flattened", frame, updateGolden)
	})

	t.Run("projects_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"id":"p1","name":"Apollo","identifier":"APO","network":2,"archived_at":null},
			{"id":"p2","name":"Beacon","identifier":"BCN","network":0,"archived_at":null}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "projects_flattened", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
