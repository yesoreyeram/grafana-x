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
// Asana responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("tasks_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"gid":"1","name":"Fix login bug","resource_type":"task","completed":false,
			 "created_at":"2024-01-02T03:04:05.000Z","modified_at":"2024-01-03T03:04:05.000Z",
			 "due_on":"2024-02-01","start_on":null,
			 "assignee":{"gid":"10","name":"Alice"},"assignee_status":"inbox",
			 "projects":[{"gid":"p1","name":"Mobile"}],
			 "parent":null,"tags":[{"gid":"t1","name":"bug"}],
			 "num_subtasks":2,"notes":"repro steps",
			 "permalink_url":"https://app.asana.com/0/1/1",
			 "custom_fields":[
			   {"gid":"cf1","name":"Priority","type":"enum","enum_value":{"name":"High"},"display_value":"High"},
			   {"gid":"cf2","name":"Story Points","type":"number","number_value":5,"display_value":"5"}
			 ]},
			{"gid":"2","name":"Write docs","resource_type":"task","completed":true,
			 "created_at":"2024-01-04T03:04:05.000Z","modified_at":"2024-01-05T03:04:05.000Z",
			 "due_on":null,"start_on":null,
			 "assignee":null,"assignee_status":"upcoming",
			 "projects":[],"parent":null,"tags":[],
			 "num_subtasks":0,"notes":"",
			 "permalink_url":"https://app.asana.com/0/1/2",
			 "custom_fields":[
			   {"gid":"cf1","name":"Priority","type":"enum","enum_value":null,"display_value":null},
			   {"gid":"cf2","name":"Story Points","type":"number","number_value":null,"display_value":null}
			 ]}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "tasks_flattened", frame, updateGolden)
	})

	t.Run("projects_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"gid":"p1","name":"Mobile App","resource_type":"project","archived":false,
			 "color":"light-green","created_at":"2024-01-01T00:00:00.000Z","modified_at":"2024-01-10T00:00:00.000Z",
			 "start_on":null,"due_on":"2024-03-01","public":true,"notes":"",
			 "owner":{"gid":"10","name":"Alice"},"team":{"gid":"tm1","name":"Engineering"},
			 "current_status":{"text":"On track"},"permalink_url":"https://app.asana.com/0/p1"},
			{"gid":"p2","name":"Website","resource_type":"project","archived":true,
			 "color":null,"created_at":"2024-02-01T00:00:00.000Z","modified_at":"2024-02-10T00:00:00.000Z",
			 "start_on":null,"due_on":null,"public":false,"notes":"redesign",
			 "owner":null,"team":{"gid":"tm1","name":"Engineering"},
			 "current_status":null,"permalink_url":"https://app.asana.com/0/p2"}
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
