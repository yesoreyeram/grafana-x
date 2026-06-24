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

func recordsFromJSON(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("invalid testdata json: %v", err)
	}
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		records = append(records, flattenRecord(item))
	}
	return records
}

// TestGolden_Frames locks down the full data-plane contract (field names, types,
// column + row order, frame metadata) for representative Pipedrive responses.
func TestGolden_Frames(t *testing.T) {
	t.Run("deals_mixed_types", func(t *testing.T) {
		records := recordsFromJSON(t, `[
			{"id":1,"title":"Big Deal","value":10000,"currency":"USD","status":"open","add_time":"2024-01-15 10:00:00","won_time":null},
			{"id":2,"title":"Won Deal","value":5000,"currency":"EUR","status":"won","add_time":"2024-02-20 12:30:00","won_time":"2024-03-01 09:00:00"}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "deals_mixed_types", frame, updateGolden)
	})

	t.Run("persons_nested_flattened", func(t *testing.T) {
		records := recordsFromJSON(t, `[
			{"id":1,"name":"Jane Doe","email":[{"label":"work","value":"jane@example.com","primary":true}],"phone":[{"label":"work","value":"+15551234","primary":true}],"org_id":{"name":"Acme","value":42},"add_time":"2024-01-10 08:00:00"}
		]`)
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "persons_nested_flattened", frame, updateGolden)
	})

	t.Run("deals_field_selection", func(t *testing.T) {
		records := recordsFromJSON(t, `[
			{"id":1,"title":"Deal A","value":100,"currency":"USD","status":"open"},
			{"id":2,"title":"Deal B","value":200,"currency":"USD","status":"open"}
		]`)
		frame := recordsToFrame("A", records, []string{"title", "value"})
		experimental.CheckGoldenJSONFrame(t, goldenDir, "deals_field_selection", frame, updateGolden)
	})

	t.Run("nulls_and_strings", func(t *testing.T) {
		records := []map[string]any{
			{"name": "a", "notes": nil, "mixed": float64(1)},
			{"name": "b", "notes": nil, "mixed": "text"},
		}
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "nulls_and_strings", frame, updateGolden)
	})

	t.Run("empty", func(t *testing.T) {
		frame := recordsToFrame("A", nil, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "empty", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
