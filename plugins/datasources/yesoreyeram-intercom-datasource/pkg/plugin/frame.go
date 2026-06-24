package plugin

import (
	"encoding/json"
	"fmt"
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

// intercomTimestampKeys lists top-level fields that Intercom returns as Unix
// epoch SECONDS (integers). Most Intercom time fields end with `_at` (handled by
// isTimestampKey); these are the exceptions that must be listed explicitly.
var intercomTimestampKeys = map[string]bool{
	"snoozed_until": true,
	"waiting_since": true,
}

// countToFrame returns a single-row, single-column frame holding a record count.
// It conforms to the data plane "numeric wide" contract so it can be used by
// stat / single-value panels and numeric-aware consumers.
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

// recordsToFrame converts a slice of flattened Intercom records into a single
// wide data.Frame conforming to the data plane "table" contract.
//
// Row order is preserved exactly as returned by Intercom, so the query sort is
// honoured. Time fields are moved to the front of the columns so that
// time-series and Explore consumers detect the time dimension, but this does NOT
// change the order of the rows themselves.
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
	fields := make([]*data.Field, len(columns))
	for i, col := range columns {
		fields[i] = buildField(col, records, rowCount, colTypes[col])
	}
	frame.Fields = fields
	return frame
}

// orderTimeFirst returns the columns with time-typed columns moved to the front,
// preserving the relative order within each group.
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

// buildField returns a populated *data.Field for a column of the given type.
// Nullable pointer types are used so missing values are represented as null,
// which is data plane compliant.
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
			if t, ok := toTime(rec[col]); ok {
				// Normalise to UTC; data plane time fields are timezone-agnostic
				// instants stored as epoch.
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

type fieldType int

const (
	fieldTypeString fieldType = iota
	fieldTypeNumber
	fieldTypeBool
	fieldTypeTime
)

// inferColumnType examines the non-null values of a column and returns the most
// specific type that fits every value. Mixed columns fall back to string.
func inferColumnType(col string, records []map[string]any) fieldType {
	hasValue := false
	allNumber, allBool, allTime := true, true, true

	for _, rec := range records {
		v, ok := rec[col]
		if !ok || v == nil {
			continue
		}
		hasValue = true

		if _, ok := toTime(v); !ok {
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
		// Objects/arrays are serialised as JSON so nested data is still visible.
		b, err := json.Marshal(s)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// timeLayouts lists the formats recognised when inferring time columns. Only
// strings that match one of these (and look date-like) are treated as time.
// Intercom timestamps are converted from epoch seconds to RFC3339 during
// flattening, so RFC3339 is the primary layout here.
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

func toTime(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok || len(s) < 8 {
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
// Intercom record flattening
//
// Intercom objects mix scalar fields with deeply nested objects/arrays, e.g.:
//
//	{
//	  "type": "conversation",
//	  "id": "123",
//	  "created_at": 1539897198,          // Unix epoch SECONDS
//	  "state": "open",
//	  "source": { "type": "conversation", "author": {...} },
//	  "tags":   { "type": "tag.list", "tags": [...] },
//	  "custom_attributes": {...}
//	}
//
// flattenIntercomRecord keeps scalar fields as-is, converts epoch-seconds
// timestamp fields to RFC3339 strings (so the shared time inference treats them
// as time fields), and serialises nested objects/arrays (source, assignee,
// contacts, tags, custom_attributes, statistics, …) to compact JSON strings so
// the data is still visible in a flat table. A synthetic id is added when the
// object has none.
// ---------------------------------------------------------------------------

func flattenIntercomRecord(raw json.RawMessage, index int) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{"id": fmt.Sprintf("row-%d", index)}
	}
	row := make(map[string]any, len(obj))
	for k, v := range obj {
		row[k] = flattenIntercomValue(k, v)
	}
	if v, ok := row["id"]; !ok || v == nil {
		row["id"] = fmt.Sprintf("row-%d", index)
	}
	return row
}

// flattenIntercomValue reduces a single field value to a scalar suitable for the
// frame builder. Objects and arrays become compact JSON strings; epoch-seconds
// timestamp fields become RFC3339 strings (0 / negative become null).
func flattenIntercomValue(key string, raw json.RawMessage) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	switch trimmed[0] {
	case '{', '[':
		// Re-marshal so nested objects serialise deterministically (Go marshals
		// map keys in sorted order), keeping golden output stable.
		var anyV any
		if err := json.Unmarshal(raw, &anyV); err != nil {
			return strings.TrimSpace(string(raw))
		}
		b, err := json.Marshal(anyV)
		if err != nil {
			return strings.TrimSpace(string(raw))
		}
		return string(b)
	}

	// Scalar value.
	if isTimestampKey(key) {
		if secs, ok := parseEpochSeconds(trimmed); ok {
			if secs <= 0 {
				return nil
			}
			return time.Unix(secs, 0).UTC().Format(time.RFC3339)
		}
	}

	var val any
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&val); err != nil {
		return nil
	}
	if n, ok := val.(json.Number); ok {
		if f, err := n.Float64(); err == nil {
			return f
		}
		return n.String()
	}
	return val
}

// isTimestampKey reports whether a top-level field is an Intercom epoch-seconds
// timestamp. Intercom names most of them with an `_at` suffix.
func isTimestampKey(k string) bool {
	if intercomTimestampKeys[k] {
		return true
	}
	return strings.HasSuffix(k, "_at")
}

// parseEpochSeconds parses a JSON numeric literal into integer seconds. Returns
// false for non-numeric values (e.g. an already-formatted date string).
func parseEpochSeconds(trimmed string) (int64, bool) {
	if trimmed == "" || trimmed[0] == '"' {
		return 0, false
	}
	n := json.Number(trimmed)
	if i, err := n.Int64(); err == nil {
		return i, true
	}
	if f, err := n.Float64(); err == nil {
		return int64(f), true
	}
	return 0, false
}
