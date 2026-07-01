package plugin

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var (
	tableTypeVersion       = data.FrameTypeVersion{0, 1}
	numericWideTypeVersion = data.FrameTypeVersion{0, 1}
)

// trelloDateKeys lists the flattened card columns whose string values are
// ISO-8601 timestamps and should be presented as time columns.
var trelloDateKeys = map[string]bool{
	"due":              true,
	"start":            true,
	"dateCreated":      true,
	"dateLastActivity": true,
}

func countToFrame(refID string, count int64) *data.Frame {
	field := data.NewField("count", nil, []int64{count})
	frame := data.NewFrame(refID, field)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeNumericWide,
		TypeVersion:            numericWideTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}
	return frame
}

func cardsToFrame(refID string, records []map[string]any) *data.Frame {
	frame := data.NewFrame(refID)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		TypeVersion:            tableTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}

	columns := orderedColumns(records)
	if len(columns) == 0 {
		return frame
	}

	colTypes := make(map[string]fieldType, len(columns))
	for _, col := range columns {
		colTypes[col] = inferColumnType(col, records)
	}
	columns = orderTimeFirst(columns, colTypes)

	rowCount := len(records)
	frameFields := make([]*data.Field, len(columns))
	for i, col := range columns {
		frameFields[i] = buildField(col, records, rowCount, colTypes[col])
	}
	frame.Fields = frameFields
	return frame
}

func orderedColumns(records []map[string]any) []string {
	seen := map[string]bool{}
	var ordered []string

	if len(records) > 0 {
		keys := make([]string, 0, len(records[0]))
		for k := range records[0] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			seen[k] = true
			ordered = append(ordered, k)
		}
	}

	var extra []string
	for _, rec := range records {
		for k := range rec {
			if !seen[k] {
				seen[k] = true
				extra = append(extra, k)
			}
		}
	}
	sort.Strings(extra)
	return append(ordered, extra...)
}

func orderTimeFirst(columns []string, colTypes map[string]fieldType) []string {
	timeCols := make([]string, 0)
	rest := make([]string, 0, len(columns))
	for _, col := range columns {
		if colTypes[col] == fieldTypeTime {
			timeCols = append(timeCols, col)
		} else {
			rest = append(rest, col)
		}
	}
	return append(timeCols, rest...)
}

type fieldType int

const (
	fieldTypeString fieldType = iota
	fieldTypeNumber
	fieldTypeBool
	fieldTypeTime
)

func inferColumnType(col string, records []map[string]any) fieldType {
	hasValue := false
	allNumber, allBool, allTime := true, true, true

	for _, rec := range records {
		v, ok := rec[col]
		if !ok || v == nil {
			continue
		}
		hasValue = true

		if _, ok := toColumnTime(col, v); !ok {
			allTime = false
		}
		if _, ok := toFloat(v); !ok {
			allNumber = false
		}
		if _, ok := toBool(v); !ok {
			allBool = false
		}
	}

	if !hasValue {
		return fieldTypeString
	}
	switch {
	case allBool:
		return fieldTypeBool
	case allTime:
		return fieldTypeTime
	case allNumber:
		return fieldTypeNumber
	default:
		return fieldTypeString
	}
}

func buildField(col string, records []map[string]any, rowCount int, ft fieldType) *data.Field {
	switch ft {
	case fieldTypeNumber:
		values := make([]*float64, rowCount)
		for i, rec := range records {
			if f, ok := toFloat(rec[col]); ok {
				v := f
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	case fieldTypeBool:
		values := make([]*bool, rowCount)
		for i, rec := range records {
			if b, ok := toBool(rec[col]); ok {
				v := b
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	case fieldTypeTime:
		values := make([]*time.Time, rowCount)
		for i, rec := range records {
			if t, ok := toColumnTime(col, rec[col]); ok {
				v := t.UTC()
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	default:
		values := make([]*string, rowCount)
		for i, rec := range records {
			if s, ok := toString(rec[col]); ok {
				v := s
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	}
}

func toColumnTime(col string, v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, false
	}
	if !trelloDateKeys[col] {
		return time.Time{}, false
	}
	if s, ok := v.(string); ok {
		return toTime(s)
	}
	return time.Time{}, false
}

func toBool(v any) (bool, bool) {
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func toString(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case nil:
		return "", false
	case bool, float64, float32, int, int64, json.Number:
		b, _ := json.Marshal(s)
		return string(b), true
	default:
		b, err := json.Marshal(s)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999Z07:00",
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999-07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func toTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return time.Time{}, false
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// flattenCard reduces a raw Trello card into a flat record of scalars. Nested
// values are mapped to their most useful representation:
//   - labels        -> comma-joined label names (falling back to colors)
//   - idMembers     -> comma-joined member ids
//   - idChecklists  -> comma-joined checklist ids
//   - badges        -> per-count scalar columns (badges_votes, badges_comments, …)
//   - customFieldItems -> compact JSON string
//
// A synthetic dateCreated column is derived from the card id (a Mongo ObjectId
// whose first 8 hex digits encode the Unix-second creation time).
func flattenCard(card map[string]any) map[string]any {
	row := make(map[string]any, len(card)+8)
	for k, v := range card {
		switch k {
		case "labels":
			row[k] = flattenLabelArray(v)
		case "idMembers", "idChecklists":
			row[k] = flattenStringArray(v)
		case "badges":
			for bk, bv := range flattenBadges(v) {
				row[bk] = bv
			}
		case "customFieldItems":
			row[k] = flattenCustomFieldItems(v)
		default:
			row[k] = v
		}
	}
	if id, ok := card["id"].(string); ok {
		if sec, ok := cardCreatedUnix(id); ok {
			row["dateCreated"] = time.Unix(sec, 0).UTC().Format(time.RFC3339)
		}
	}
	return row
}

// cardCreatedUnix extracts the Unix-second creation time embedded in a Trello
// card id. The id is a Mongo ObjectId whose first 8 hex characters are the
// creation timestamp.
func cardCreatedUnix(id string) (int64, bool) {
	id = strings.TrimSpace(id)
	if len(id) < 8 {
		return 0, false
	}
	sec, err := strconv.ParseInt(id[:8], 16, 64)
	if err != nil {
		return 0, false
	}
	return sec, true
}

func flattenLabelArray(v any) any {
	if v == nil {
		return nil
	}
	labels, ok := v.([]any)
	if !ok {
		return v
	}
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		if obj, ok := l.(map[string]any); ok {
			if name, ok := obj["name"].(string); ok && name != "" {
				names = append(names, name)
			} else if color, ok := obj["color"].(string); ok && color != "" {
				names = append(names, color)
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return strings.Join(names, ", ")
}

func flattenStringArray(v any) any {
	if v == nil {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return v
	}
	strs := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			strs = append(strs, s)
		}
	}
	if len(strs) == 0 {
		return nil
	}
	return strings.Join(strs, ", ")
}

// flattenBadges extracts the numeric count fields from a card's badges object
// into discrete scalar columns (e.g. badges_votes, badges_comments). Non-count
// badge fields (booleans, due dates) are intentionally omitted; the canonical
// due/start values are already top-level card fields.
func flattenBadges(v any) map[string]any {
	out := map[string]any{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for _, key := range []string{"votes", "comments", "attachments", "checkItems", "checkItemsChecked"} {
		if n, ok := toFloat(m[key]); ok {
			out["badges_"+key] = n
		}
	}
	return out
}

func flattenCustomFieldItems(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
