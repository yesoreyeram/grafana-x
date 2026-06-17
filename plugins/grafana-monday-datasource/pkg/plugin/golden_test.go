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

func flattenItems(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var nodes []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		t.Fatal(err)
	}
	records := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		records = append(records, flattenItem(n, true, false))
	}
	return records
}

func flattenNodes(t *testing.T, raw string) []map[string]any {
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
// monday.com responses match the committed golden files. These lock down the
// full data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	t.Run("items_flattened", func(t *testing.T) {
		records := flattenItems(t, `[
			{"id":"11","name":"Fix login","state":"active",
			 "created_at":"2024-01-15T10:00:00Z","updated_at":"2024-01-16T10:00:00Z",
			 "group":{"id":"g1","title":"Doing"},"board":{"id":"1","name":"Tasks"},
			 "column_values":[{"id":"status","type":"status","column":{"title":"Status"},"text":"Working"},
			                  {"id":"owner","type":"people","column":{"title":"Owner"},"text":"Alice"},
			                  {"id":"done","type":"checkbox","column":{"title":"Done"},"text":""}]},
			{"id":"12","name":"Add SSO","state":"active",
			 "created_at":"2024-01-14T09:00:00Z","updated_at":"2024-01-15T09:00:00Z",
			 "group":{"id":"g1","title":"Doing"},"board":{"id":"1","name":"Tasks"},
			 "column_values":[{"id":"status","type":"status","column":{"title":"Status"},"text":"Done"},
			                  {"id":"owner","type":"people","column":{"title":"Owner"},"text":"Bob"},
			                  {"id":"done","type":"checkbox","column":{"title":"Done"},"text":"v"}]}
		]`)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "items_flattened", frame, updateGolden)
	})

	t.Run("items_hide_system_columns", func(t *testing.T) {
		raw := `[
			{"id":"11","name":"Fix login","state":"active",
			 "created_at":"2024-01-15T10:00:00Z",
			 "column_values":[{"id":"status","type":"status","column":{"title":"Status"},"text":"Working"},
			                  {"id":"subitems","type":"subtasks","column":{"title":"Subitems"},"text":"2 items"},
			                  {"id":"updated","type":"last_updated","column":{"title":"Last updated"},"text":"2024-01-16"}]}
		]`
		var nodes []json.RawMessage
		if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
			t.Fatal(err)
		}
		records := make([]map[string]any, 0, len(nodes))
		for _, n := range nodes {
			records = append(records, flattenItem(n, true, true))
		}
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "items_hide_system_columns", frame, updateGolden)
	})

	t.Run("boards_flattened", func(t *testing.T) {
		records := flattenNodes(t, `[
			{"id":"1","name":"Tasks","state":"active","items_count":12,
			 "updated_at":"2024-01-16T10:00:00Z","workspace":{"id":"w1","name":"Main"}},
			{"id":"2","name":"Bugs","state":"active","items_count":4,
			 "updated_at":"2024-01-10T10:00:00Z","workspace":{"id":"w1","name":"Main"}}
		]`)
		frame := recordsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "boards_flattened", frame, updateGolden)
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
