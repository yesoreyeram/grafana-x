package plugin

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

// updateGolden controls whether golden files are (re)written. Regenerate with:
//
//	UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
//
// Then review and commit the files under pkg/plugin/testdata/.
var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

func flattenStoriesRaw(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatal(err)
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, flattenStory(item))
	}
	return records
}

// TestGolden_Frames locks the full data-plane contract (field names, types,
// column + row order, frame metadata) for representative Shortcut search
// responses.
func TestGolden_Frames(t *testing.T) {
	// A representative pair of search stories with the real search shape:
	// scalar *_id fields, owner_ids/label_ids arrays, ISO-8601 times.
	const storiesJSON = `[
		{"id":1,"name":"Fix login","story_type":"bug","estimate":3,
		 "workflow_state_id":100,"project_id":10,"epic_id":5,"iteration_id":3,
		 "group_id":"g-1","requested_by_id":"u-9",
		 "owner_ids":["u-1","u-2"],"label_ids":[1,2],
		 "archived":false,"started":true,"completed":false,"blocked":false,"blocker":false,
		 "num_tasks_completed":1,"position":1,
		 "created_at":"2024-01-15T10:00:00Z","updated_at":"2024-01-16T11:00:00Z",
		 "started_at":"2024-01-15T12:00:00Z","completed_at":null,"deadline":"2024-02-01T00:00:00Z",
		 "moved_at":"2024-01-16T11:00:00Z","description":"Login is broken",
		 "app_url":"https://app.shortcut.com/story/1"},
		{"id":2,"name":"Add SSO","story_type":"feature","estimate":5,
		 "workflow_state_id":101,"project_id":10,"epic_id":5,"iteration_id":3,
		 "group_id":"g-1","requested_by_id":"u-9",
		 "owner_ids":["u-2"],"label_ids":[3],
		 "archived":false,"started":false,"completed":false,"blocked":false,"blocker":false,
		 "num_tasks_completed":0,"position":2,
		 "created_at":"2024-01-14T09:00:00Z","updated_at":"2024-01-14T09:00:00Z",
		 "started_at":null,"completed_at":null,"deadline":null,
		 "moved_at":"2024-01-14T09:00:00Z","description":"Single sign-on",
		 "app_url":"https://app.shortcut.com/story/2"}
	]`

	t.Run("stories_flattened", func(t *testing.T) {
		records := flattenStoriesRaw(t, storiesJSON)
		frame := recordsToFrame("A", records, effectiveFields(nil))
		experimental.CheckGoldenJSONFrame(t, goldenDir, "stories_flattened", frame, updateGolden)
	})

	t.Run("stories_field_selection", func(t *testing.T) {
		records := flattenStoriesRaw(t, storiesJSON)
		frame := recordsToFrame("A", records, []string{"id", "name", "created_at"})
		experimental.CheckGoldenJSONFrame(t, goldenDir, "stories_field_selection", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil, effectiveFields(nil))
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
