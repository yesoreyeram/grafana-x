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
		records = append(records, flattenTask(n))
	}
	return records
}

// TestGolden_Frames asserts that the data frames produced for representative
// ClickUp responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("tasks_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"id":"9hx","name":"Fix login bug","archived":false,
			 "status":{"status":"in progress","type":"custom"},
			 "priority":{"priority":"urgent","id":"1"},
			 "points":3,
			 "date_created":"1567780450202","date_updated":"1567780460000","due_date":"1508369194377",
			 "creator":{"username":"Alex"},
			 "assignees":[{"username":"Alice"},{"username":"Bob"}],
			 "tags":[{"name":"bug"}],
			 "list":{"name":"Sprint Backlog"},"space":{"id":"7"},
			 "url":"https://app.clickup.com/t/9hx"},
			{"id":"9hz","name":"Update docs","archived":false,
			 "status":{"status":"to do","type":"open"},
			 "priority":null,
			 "points":null,
			 "date_created":"1567780450500","date_updated":"1567780470000","due_date":null,
			 "creator":{"username":"Alex"},
			 "assignees":[],
			 "tags":[],
			 "list":{"name":"Sprint Backlog"},"space":{"id":"7"},
			 "url":"https://app.clickup.com/t/9hz"}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "tasks_flattened", frame, updateGolden)
	})

	t.Run("spaces_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"id":"s1","name":"Engineering","private":false,"archived":false,"multiple_assignees":true},
			{"id":"s2","name":"Operations","private":true,"archived":false,"multiple_assignees":false}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "spaces_flattened", frame, updateGolden)
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
