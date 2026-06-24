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

func decodeItems(t *testing.T, raw string) []json.RawMessage {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("invalid testdata json: %v", err)
	}
	return items
}

// TestGolden_Frames asserts that the data frames produced for representative
// Confluence responses match the committed golden files. These lock down the
// full data-plane contract: field names, field types (time/number/bool/string),
// column ordering, row order and frame metadata.
func TestGolden_Frames(t *testing.T) {
	const origin = "https://acme.atlassian.net"

	t.Run("records_flattened_pages", func(t *testing.T) {
		items := decodeItems(t, `[
			{"id":"123","status":"current","title":"Release notes","spaceId":"456","authorId":"u1",
			 "createdAt":"2024-01-02T03:04:05.000Z",
			 "version":{"number":3,"message":"edit","createdAt":"2024-02-02T03:04:05.000Z","authorId":"u2"},
			 "_links":{"webui":"/spaces/ENG/pages/123/Release"}},
			{"id":"124","status":"current","title":"Onboarding","spaceId":"456","authorId":"u3",
			 "createdAt":"2024-01-03T03:04:05.000Z",
			 "version":{"number":1,"message":"","createdAt":"2024-01-03T03:04:05.000Z","authorId":"u3"},
			 "_links":{"webui":"/spaces/ENG/pages/124/Onboarding"}}
		]`)
		frame := recordsToFrame("A", flattenContentItems(items, origin), nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_flattened_pages", frame, updateGolden)
	})

	t.Run("records_search", func(t *testing.T) {
		items := decodeItems(t, `[
			{"content":{"id":"123","type":"page","status":"current","title":"Release notes","spaceId":"456"},
			 "title":"Release @@@hl@@@notes@@@endhl@@@","excerpt":"some text","url":"/spaces/ENG/pages/123",
			 "lastModified":"2024-03-01T10:00:00.000Z"},
			{"content":{"id":"900","type":"blogpost","status":"current","title":"Announcement","spaceId":"456"},
			 "title":"Announcement","excerpt":"hello","url":"/spaces/ENG/blog/900",
			 "lastModified":"2024-02-15T09:00:00.000Z"}
		]`)
		frame := recordsToFrame("A", flattenSearchItems(items, origin), nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_search", frame, updateGolden)
	})

	t.Run("records_mixed_types", func(t *testing.T) {
		records := []map[string]any{
			{"id": "1", "title": "Alice", "versionNumber": float64(2), "createdAt": "2024-01-15T00:00:00.000Z"},
			{"id": "2", "title": "Bob", "versionNumber": float64(5), "createdAt": "2024-02-20T00:00:00.000Z"},
		}
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_mixed_types", frame, updateGolden)
	})

	t.Run("records_nulls_and_strings", func(t *testing.T) {
		records := []map[string]any{
			{"title": "a", "excerpt": nil, "versionNumber": float64(1)},
			{"title": "b", "excerpt": nil, "versionNumber": nil},
		}
		frame := recordsToFrame("A", records, nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_nulls_and_strings", frame, updateGolden)
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
