package plugin

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/experimental"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "true"

const goldenDir = "testdata"

func makeCardsRecords(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var cards []map[string]any
	if err := json.Unmarshal([]byte(raw), &cards); err != nil {
		t.Fatal(err)
	}
	result := make([]map[string]any, 0, len(cards))
	for _, card := range cards {
		result = append(result, flattenCard(card))
	}
	return result
}

func TestGolden_Frames(t *testing.T) {
	t.Run("cards_flattened", func(t *testing.T) {
		records := makeCardsRecords(t, `[
			{"id":"c1","name":"Fix login bug","desc":"Investigate 500 error","closed":false,"pos":1,
			 "due":"2024-02-01T00:00:00Z","dateLastActivity":"2024-01-15T10:30:00Z",
			 "idList":"l1","idBoard":"b1","shortUrl":"https://trello.com/c/abc",
			 "idMembers":["u1","u2"],
			 "labels":[{"id":"l1","name":"bug","color":"red"},{"id":"l2","name":"urgent","color":"orange"}],
			 "start":"2024-01-10T00:00:00Z"},
			{"id":"c2","name":"Update docs","desc":"Write README","closed":false,"pos":2,
			 "due":null,"dateLastActivity":"2024-01-14T09:00:00Z",
			 "idList":"l2","idBoard":"b1","shortUrl":"https://trello.com/c/def",
			 "idMembers":[],"labels":[],"start":null}
		]`)
		frame := cardsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "cards_flattened", frame, updateGolden)
	})

	t.Run("cards_full", func(t *testing.T) {
		// Realistic ObjectId-style ids so dateCreated is derived; badges expand to
		// per-count columns; checklists/custom fields are flattened.
		records := makeCardsRecords(t, `[
			{"id":"5e8f1a2b3c4d5e6f7a8b9c0d","name":"Ship release","desc":"Cut v2","closed":false,"pos":16384,
			 "due":"2024-03-01T17:00:00Z","dueComplete":false,"start":"2024-02-20T00:00:00Z",
			 "dateLastActivity":"2024-02-25T08:15:00Z","idList":"l1","idBoard":"b1",
			 "shortUrl":"https://trello.com/c/aaa","url":"https://trello.com/c/aaa/ship-release",
			 "idMembers":["u1","u2"],"idChecklists":["chk1"],
			 "labels":[{"id":"lab1","name":"release","color":"green"}],
			 "badges":{"votes":3,"comments":5,"attachments":2,"checkItems":4,"checkItemsChecked":1,"subscribed":true},
			 "customFieldItems":[{"idCustomField":"cf1","value":{"text":"High"}}]},
			{"id":"5e7c0a1b3c4d5e6f7a8b9c0e","name":"Triage bugs","desc":"","closed":true,"pos":32768,
			 "due":null,"dueComplete":false,"start":null,
			 "dateLastActivity":"2024-02-24T10:00:00Z","idList":"l2","idBoard":"b1",
			 "shortUrl":"https://trello.com/c/bbb","url":"https://trello.com/c/bbb/triage-bugs",
			 "idMembers":[],"idChecklists":[],"labels":[],
			 "badges":{"votes":0,"comments":0,"attachments":0,"checkItems":0,"checkItemsChecked":0},
			 "customFieldItems":[]}
		]`)
		frame := cardsToFrame("A", records)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "cards_full", frame, updateGolden)
	})

	t.Run("records_empty", func(t *testing.T) {
		frame := cardsToFrame("A", nil)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "records_empty", frame, updateGolden)
	})

	t.Run("count", func(t *testing.T) {
		frame := countToFrame("A", 42)
		experimental.CheckGoldenJSONFrame(t, goldenDir, "count", frame, updateGolden)
	})
}
