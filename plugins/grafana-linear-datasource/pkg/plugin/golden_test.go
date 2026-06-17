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
	var nodes []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		t.Fatal(err)
	}
	records := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		records = append(records, flattenNode(n))
	}
	return records
}

// TestGolden_Frames asserts that the data frames produced for representative
// Linear responses match the committed golden files. These lock down the full
// data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("issues_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"identifier":"ENG-1","title":"Fix login","priority":2,"estimate":3,
			 "createdAt":"2024-01-15T10:00:00.000Z","state":{"name":"In Progress","type":"started"},
			 "assignee":{"name":"Alice"},"team":{"key":"ENG","name":"Engineering"},
			 "labels":{"nodes":[{"name":"bug"},{"name":"p1"}]}},
			{"identifier":"ENG-2","title":"Add SSO","priority":1,"estimate":5,
			 "createdAt":"2024-01-14T09:00:00.000Z","state":{"name":"Todo","type":"unstarted"},
			 "assignee":{"name":"Bob"},"team":{"key":"ENG","name":"Engineering"},
			 "labels":{"nodes":[{"name":"feature"}]}}
		]`)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "issues_flattened", frame, updateGolden)
	})

	t.Run("projects_flattened", func(t *testing.T) {
		records := flattenRaw(t, `[
			{"name":"Mobile App","state":"started","progress":0.42,
			 "startDate":"2024-01-01","targetDate":"2024-06-30",
			 "createdAt":"2023-12-01T00:00:00.000Z","lead":{"name":"Carol"}},
			{"name":"API v2","state":"planned","progress":0,
			 "startDate":"2024-02-01","targetDate":"2024-09-30",
			 "createdAt":"2024-01-05T00:00:00.000Z","lead":{"name":"Dan"}}
		]`)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "projects_flattened", frame, updateGolden)
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
