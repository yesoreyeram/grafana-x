package plugin

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Data plane contract type versions. See https://grafana.github.io/dataplane/contract/
var (
	tableTypeVersion       = data.FrameTypeVersion{0, 1}
	numericWideTypeVersion = data.FrameTypeVersion{0, 1}
)

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

// recordsToFrame converts a slice of flattened Grist records (flat JSON maps
// keyed by column name, including the row `id`) into a single wide data.Frame
// conforming to the data plane "table" contract.
//
// dateCols carries the set of columns whose Grist type is Date/DateTime. Grist
// returns those columns as Unix epoch *seconds* (numbers), so they are converted
// to UTC time fields here — the conversion is driven by column metadata, NOT by
// sniffing string formats.
//
// Row order is preserved exactly as returned by Grist, so the query's sort is
// honoured. Time fields are moved to the front of the columns so time-series and
// Explore consumers detect the time dimension, but this does NOT change the
// order of the rows themselves.
func recordsToFrame(refID string, records []map[string]any, dateCols map[string]bool) *data.Frame {
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

	// Determine the type of each column once. Date/DateTime columns are forced
	// to time (driven by metadata); the rest are inferred from their values.
	colTypes := make(map[string]fieldType, len(columns))
	for _, col := range columns {
		if dateCols[col] {
			colTypes[col] = fieldTypeTime
		} else {
			colTypes[col] = inferColumnType(col, records)
		}
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

// orderedColumns returns a stable, deterministic column order across records:
// the union of column names sorted alphabetically.
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
// specific scalar type that fits every value. Mixed columns fall back to string.
//
// Time is NOT inferred here: Grist Date/DateTime columns arrive as epoch-seconds
// numbers and are indistinguishable from ordinary numbers without column
// metadata, so the time decision is made by the caller via dateCols.
func inferColumnType(col string, records []map[string]any) fieldType {
	hasValue := false
	allNumber, allBool := true, true

	for _, rec := range records {
		v, ok := rec[col]
		if !ok || v == nil {
			continue
		}
		hasValue = true

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
		// Objects/arrays (Choice lists, reference lists, Grist's typed
		// ["L", ...] / ["R", ...] cell encodings, etc.) are serialised as JSON so
		// nested data is still visible rather than silently dropped.
		b, err := json.Marshal(s)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// timeLayouts lists the string formats recognised as a fallback when a Date/
// DateTime column happens to carry an ISO string rather than an epoch number
// (e.g. via a raw SQL alias). Grist's normal encoding is epoch seconds, handled
// numerically in toTime.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// toTime converts a Grist Date/DateTime cell value to a time.Time. Grist returns
// these as Unix epoch *seconds* (numbers), which is the primary path; ISO-8601
// strings are accepted as a fallback. Null/empty values yield (zero, false).
func toTime(v any) (time.Time, bool) {
	switch val := v.(type) {
	case nil:
		return time.Time{}, false
	case string:
		if len(val) < 8 {
			return time.Time{}, false
		}
		for _, layout := range timeLayouts {
			if t, err := time.Parse(layout, val); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	default:
		f, ok := toFloat(v)
		if !ok {
			return time.Time{}, false
		}
		// Epoch seconds (Grist Date is midnight UTC; DateTime has second
		// precision). Preserve any fractional component as nanoseconds.
		sec := math.Floor(f)
		nsec := (f - sec) * 1e9
		return time.Unix(int64(sec), int64(math.Round(nsec))).UTC(), true
	}
}
