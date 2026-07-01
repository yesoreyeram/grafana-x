package plugin

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Data plane contract type versions. See https://grafana.github.io/dataplane/contract/
var (
	tableTypeVersion       = data.FrameTypeVersion{0, 1}
	numericWideTypeVersion = data.FrameTypeVersion{0, 1}
)

// dateKeys lists the Shortcut story fields whose values are ISO-8601 / RFC3339
// timestamp strings and should be presented as time columns. Only these columns
// are parsed as time; other string columns (e.g. a story name that happens to
// look like a date) stay strings.
var dateKeys = map[string]bool{
	"created_at":            true,
	"updated_at":            true,
	"started_at":            true,
	"completed_at":          true,
	"moved_at":              true,
	"deadline":              true,
	"started_at_override":   true,
	"completed_at_override": true,
}

// countToFrame returns a single-row, single-column frame holding a record count,
// conforming to the data plane "numeric wide" contract.
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

// recordsToFrame converts flattened Shortcut story records into a single wide
// data.Frame conforming to the data plane "table" contract.
//
// Row order is preserved exactly as returned by Shortcut (its search ranking).
// Time fields are moved to the front of the columns so time-series and Explore
// consumers detect the time dimension, but the row order itself is untouched.
func recordsToFrame(refID string, records []map[string]any, fields []string) *data.Frame {
	frame := data.NewFrame(refID)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		TypeVersion:            tableTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}

	columns := orderedColumns(records)
	columns = selectColumns(columns, fields)
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

// selectColumns restricts the column set to the requested fields (in the column
// order), keeping only those that exist. An empty/nil selection returns all.
func selectColumns(columns []string, fields []string) []string {
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

// inferColumnType examines a column's non-null values and returns the most
// specific type fitting every value. Known Shortcut date columns are treated as
// time; mixed columns fall back to string.
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

// toColumnTime interprets a value as time for the given column. Only known date
// columns are parsed; non-date columns are never treated as time.
func toColumnTime(col string, v any) (time.Time, bool) {
	if v == nil || !dateKeys[col] {
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

// timeLayouts lists the formats recognised when parsing Shortcut timestamps.
// Shortcut returns RFC3339 timestamps (e.g. "2016-12-31T12:30:00Z").
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05.000-07:00",
	"2006-01-02T15:04:05-07:00",
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

// buildField returns a populated *data.Field for a column of the given type.
// Nullable pointer types represent missing values as null (data plane compliant).
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

// ---------------------------------------------------------------------------
// Shortcut story flattening
//
// A Shortcut story is a nested JSON object. flattenStory reduces it to a flat
// record keyed by field name: scalars pass through (numbers as float64, bools,
// strings), and nested arrays/objects (owner_ids, label_ids, labels,
// custom_fields, stats, …) are preserved losslessly as compact JSON strings so
// no data is dropped while the record still flows through the shared frame
// builder.
// ---------------------------------------------------------------------------

// flattenStory converts a single Shortcut story object into a flat record.
func flattenStory(raw json.RawMessage) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	row := make(map[string]any, len(obj))
	for key, value := range obj {
		row[key] = flattenStoryValue(value)
	}
	return row
}

// flattenStoryValue reduces a story field value to a frame cell: a scalar for
// JSON scalars, or a compact JSON string for arrays/objects.
func flattenStoryValue(raw json.RawMessage) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	switch trimmed[0] {
	case '{', '[':
		if isEmptyComposite(trimmed) {
			return nil
		}
		return compactJSON(raw)
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

// isEmptyComposite reports whether a JSON array/object literal is empty ([] or
// {}), in which case it is rendered as null rather than "[]"/"{}".
func isEmptyComposite(trimmed string) bool {
	return trimmed == "[]" || trimmed == "{}"
}

// compactJSON returns the value re-encoded as a compact (whitespace-free) JSON
// string.
func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return buf.String()
}
