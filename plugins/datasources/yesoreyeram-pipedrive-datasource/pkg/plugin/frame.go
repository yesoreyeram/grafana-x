package plugin

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Data plane contract type versions. See https://grafana.github.io/dataplane/contract/
var (
	tableTypeVersion       = data.FrameTypeVersion{0, 1}
	numericWideTypeVersion = data.FrameTypeVersion{0, 1}
)

// pipedriveDateKeys lists Pipedrive standard field names that are date/time
// values. Pipedrive v1 returns datetimes as "2006-01-02 15:04:05" (UTC) and
// dates as "2006-01-02". Columns whose names match these keys (or end with
// "_time"/"_date") are parsed to time fields.
var pipedriveDateKeys = map[string]bool{
	"add_time":            true,
	"update_time":         true,
	"close_time":          true,
	"won_time":            true,
	"lost_time":           true,
	"stage_change_time":   true,
	"expected_close_date": true,
	"next_activity_date":  true,
	"last_activity_date":  true,
	"first_won_time":      true,
	"marketing_status":    false,
	"created":             true,
	"modified":            true,
}

// countToFrame returns a single-row, single-column numeric-wide frame holding a
// record count.
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

// recordsToFrame converts flattened Pipedrive records into a single wide
// data.Frame conforming to the data plane "table" contract. Row order is
// preserved exactly as returned by the API; only columns are reordered so that
// time fields appear first.
func recordsToFrame(refID string, records []map[string]any, fields []string) *data.Frame {
	frame := data.NewFrame(refID)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		TypeVersion:            tableTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}
	columns := orderedColumns(records)
	columns = selectFrameColumns(columns, fields)
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

func selectFrameColumns(columns []string, fields []string) []string {
	want := map[string]bool{}
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			want[f] = true
		}
	}
	if len(want) == 0 {
		return columns
	}
	out := make([]string, 0, len(columns))
	for _, col := range columns {
		if want[col] {
			out = append(out, col)
		}
	}
	return out
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

// buildField returns a populated nullable *data.Field for a column. Nullable
// pointer types represent missing values as null, which is data plane compliant.
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

// toColumnTime parses a value as time only when the column name is recognised as
// a date/time field. This avoids treating arbitrary date-like strings (e.g. a
// note containing a date) as time columns.
func toColumnTime(col string, v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, false
	}
	if !pipedriveDateKeys[col] && !strings.HasSuffix(col, "_time") && !strings.HasSuffix(col, "_date") {
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

// timeLayouts lists the formats recognised when inferring time columns. Pipedrive
// v1 uses space-separated UTC datetimes and date-only values.
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
	if len(s) < 10 {
		return time.Time{}, false
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ---------------------------------------------------------------------------
// Pipedrive record flattening
//
// Pipedrive returns mostly flat records, but some fields are nested:
//   - relation fields (person_id, org_id, user_id, owner_id) are objects with a
//     "name" and a "value" (the related entity id);
//   - persons' "email" and "phone" are arrays of {label, value, primary}.
//
// flattenRecord reduces each field to a scalar (or a compact comma-joined
// string) so records flow through the shared type-inference / frame builder.
// ---------------------------------------------------------------------------

func flattenRecord(raw json.RawMessage) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	row := make(map[string]any, len(obj))
	for k, v := range obj {
		row[k] = flattenValue(v)
	}
	return row
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

// flattenObject reduces a nested object to its most meaningful scalar. The key
// preference puts "value" ahead of "label" so person email/phone entries
// (which have no "name") resolve to the email/phone value rather than the label,
// while relation objects (which have "name") still resolve to the readable name.
func flattenObject(raw json.RawMessage) any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"name", "display_name", "value", "label", "title", "id"} {
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
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", false
	}
	if trimmed[0] != '"' {
		if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return trimmed, true
		}
		if trimmed == "true" || trimmed == "false" {
			return trimmed, true
		}
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	if strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}
