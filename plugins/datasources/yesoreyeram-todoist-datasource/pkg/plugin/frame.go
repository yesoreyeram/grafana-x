package plugin

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var (
	tableTypeVersion       = data.FrameTypeVersion{0, 1}
	numericWideTypeVersion = data.FrameTypeVersion{0, 1}
)

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

func recordsToFrame(refID string, records []map[string]any) *data.Frame {
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

type fieldType int

const (
	fieldTypeString fieldType = iota
	fieldTypeNumber
	fieldTypeBool
	fieldTypeTime
)

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
			if t, ok := toColumnTime(rec[col]); ok {
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

func inferColumnType(col string, records []map[string]any) fieldType {
	hasValue := false
	allNumber, allBool, allTime := true, true, true

	for _, rec := range records {
		v, ok := rec[col]
		if !ok || v == nil {
			continue
		}
		hasValue = true

		if _, ok := toColumnTime(v); !ok {
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

func toColumnTime(v any) (time.Time, bool) {
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
	"2006-01-02T15:04:05.000-07:00",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
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

// flattenItem converts a single Todoist v1 resource object (a task) into a flat
// record keyed by field name. The nested due/deadline/duration objects are
// expanded into dedicated scalar columns and the labels array is serialised.
//
// Todoist v1 field notes (renamed from the older REST v2):
//   - checked         (was is_completed)
//   - added_at        (was created_at)
//   - added_by_uid    (was creator_id)
//   - responsible_uid (was assignee_id)
//   - assigned_by_uid (was assigner_id)
//   - child_order     (was order)
//   - note_count      (was comment_count)
//   - url             removed (task URL is https://app.todoist.com/app/task/<id>)
func flattenItem(raw json.RawMessage) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	row := make(map[string]any, len(obj))
	for key, value := range obj {
		switch key {
		case "due":
			flattenDue(value, row)
		case "deadline":
			flattenDeadline(value, row)
		case "duration":
			flattenDuration(value, row)
		case "labels":
			row[key] = flattenStringArray(value)
		default:
			row[key] = flattenValue(value)
		}
	}
	return row
}

// flattenDue expands a Todoist v1 due object into scalar columns. In v1 the due
// object is {date, timezone, string, lang, is_recurring} — there is no separate
// `datetime` field; `date` holds either a full-day date ("2024-01-15"), a
// floating datetime ("2024-01-15T12:00:00") or a fixed-timezone datetime
// ("2024-01-15T12:00:00Z"). The frame builder infers it as a time column.
func flattenDue(raw json.RawMessage, row map[string]any) {
	if raw == nil || strings.TrimSpace(string(raw)) == "null" {
		return
	}
	var due struct {
		Date        string `json:"date"`
		String      string `json:"string"`
		Timezone    string `json:"timezone"`
		IsRecurring bool   `json:"is_recurring"`
	}
	if err := json.Unmarshal(raw, &due); err != nil {
		return
	}
	if due.Date != "" {
		row["dueDate"] = due.Date
	}
	if due.String != "" {
		row["dueString"] = due.String
	}
	row["dueIsRecurring"] = due.IsRecurring
	if due.Timezone != "" {
		row["dueTimezone"] = due.Timezone
	}
}

// flattenDeadline expands a Todoist v1 deadline object {date, lang} into a
// deadlineDate column (a full-day date, "YYYY-MM-DD").
func flattenDeadline(raw json.RawMessage, row map[string]any) {
	if raw == nil || strings.TrimSpace(string(raw)) == "null" {
		return
	}
	var deadline struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(raw, &deadline); err != nil {
		return
	}
	if deadline.Date != "" {
		row["deadlineDate"] = deadline.Date
	}
}

func flattenDuration(raw json.RawMessage, row map[string]any) {
	if raw == nil || strings.TrimSpace(string(raw)) == "null" {
		return
	}
	var dur struct {
		Amount int64  `json:"amount"`
		Unit   string `json:"unit"`
	}
	if err := json.Unmarshal(raw, &dur); err != nil {
		return
	}
	row["durationAmount"] = dur.Amount
	row["durationUnit"] = dur.Unit
}

func flattenStringArray(raw json.RawMessage) any {
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	if len(items) == 0 {
		return nil
	}
	b, _ := json.Marshal(items)
	return string(b)
}

func flattenValue(raw json.RawMessage) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	switch trimmed[0] {
	case '{':
		return flattenObject(raw)
	case '[':
		return flattenArray(raw)
	default:
		var v any
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			return nil
		}
		if n, ok := v.(json.Number); ok {
			if f, err := n.Float64(); err == nil {
				return f
			}
			return n.String()
		}
		return v
	}
}

func flattenObject(raw json.RawMessage) any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"name", "title", "text", "username", "id"} {
		if v, ok := obj[key]; ok {
			if s, ok := stringScalar(v); ok {
				return s
			}
		}
	}
	return string(raw)
}

func flattenArray(raw json.RawMessage) any {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		switch v := flattenValue(item).(type) {
		case nil:
			continue
		case string:
			if v != "" {
				names = append(names, v)
			}
		default:
			if s, ok := toString(v); ok {
				names = append(names, s)
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return strings.Join(names, ", ")
}

func stringScalar(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	if strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}
