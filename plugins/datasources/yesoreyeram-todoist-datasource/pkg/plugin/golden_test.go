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
		records = append(records, flattenItem(n))
	}
	return records
}

// TestGolden_Frames asserts that the data frames produced for representative
// Todoist v1 task responses match the committed golden files. These lock down
// the full data-plane contract: field names, field types (time/number/bool/
// string), column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("tasks_flattened", func(t *testing.T) {
		// First task: fully populated, including a due object with a
		// fixed-timezone datetime, a deadline, a duration and labels.
		// Second task: minimal, with due/deadline/duration null and no labels.
		records := flattenRaw(t, `[
			{"id":"6XGgmFVcrG5RRjVr","content":"Fix login bug","description":"Need to fix auth","checked":false,
			 "priority":4,"child_order":1,
			 "added_at":"2024-01-02T03:04:05.000Z","updated_at":"2024-01-03T03:04:05.000Z",
			 "project_id":"p1","section_id":"s1","parent_id":null,
			 "added_by_uid":"u1","responsible_uid":"u2","assigned_by_uid":"u3","note_count":2,
			 "is_collapsed":false,"is_deleted":false,
			 "labels":["bug","urgent"],
			 "due":{"date":"2024-01-15T12:00:00Z","string":"Jan 15 12pm","is_recurring":false,"timezone":"Europe/Madrid","lang":"en"},
			 "deadline":{"date":"2024-01-20","lang":"en"},
			 "duration":{"amount":30,"unit":"minute"}},
			{"id":"6fFPHV272WWh3gpW","content":"Write docs","description":"","checked":true,
			 "priority":1,"child_order":2,
			 "added_at":"2024-01-04T03:04:05.000Z","updated_at":"2024-01-05T03:04:05.000Z",
			 "project_id":"p1","section_id":null,"parent_id":null,
			 "added_by_uid":"u1","responsible_uid":null,"assigned_by_uid":null,"note_count":0,
			 "is_collapsed":false,"is_deleted":false,
			 "labels":[],
			 "due":null,"deadline":null,"duration":null}
		]`)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "tasks_flattened", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
