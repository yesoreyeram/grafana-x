package plugin

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Data plane contract type versions. See https://grafana.github.io/dataplane/contract/
var (
	tableTypeVersion       = data.FrameTypeVersion{0, 1}
	numericWideTypeVersion = data.FrameTypeVersion{0, 1}
)

// identityColumns are the PocketBase system fields promoted to the front of each
// frame, in this order. Every record carries `id`; base collections also carry
// the `created`/`updated` autodates by default (when present).
var identityColumns = []string{"id", "created", "updated"}

// systemColumns is the set of PocketBase system fields dropped from the frame
// when hideSystemFields is set.
var systemColumns = map[string]bool{
	"id":             true,
	"collectionId":   true,
	"collectionName": true,
	"created":        true,
	"updated":        true,
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

// recordsToFrame converts a slice of PocketBase records (flat JSON maps,
// including the system fields id/collectionId/collectionName/created/updated)
// into a single wide data.Frame conforming to the data plane "table" contract.
//
// Row order is preserved exactly as returned by PocketBase, so the query's sort
// (or the default order) is honoured. Time columns are moved to the front so
// time-series and Explore consumers detect the time dimension, but this does NOT
// change the order of the rows themselves.
//
// When hideSystemFields is true, the PocketBase system columns (id,
// collectionId, collectionName, created, updated) are dropped from the frame.
func recordsToFrame(refID string, records []map[string]any, hideSystemFields bool) *data.Frame {
	frame := data.NewFrame(refID)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		TypeVersion:            tableTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}

	columns := orderedColumns(records)
	if hideSystemFields {
		columns = dropSystemColumns(columns)
	}
	if len(columns) == 0 {
		return frame
	}

	// Determine the type of each column once, then order time columns first so
	// the frame presents its time dimension up front.
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

// dropSystemColumns removes the PocketBase system columns from the column list,
// preserving the order of the remaining columns.
func dropSystemColumns(columns []string) []string {
	out := make([]string, 0, len(columns))
	for _, col := range columns {
		if systemColumns[col] {
			continue
		}
		out = append(out, col)
	}
	return out
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

// orderedColumns returns a stable, deterministic column order across records.
// The PocketBase system identity columns (id, created, updated) lead, followed
// by the union of the remaining field names sorted alphabetically.
func orderedColumns(records []map[string]any) []string {
	seen := map[string]bool{}
	var ordered []string

	// Identity columns first, when present in any record.
	for _, id := range identityColumns {
		for _, rec := range records {
			if _, ok := rec[id]; ok {
				seen[id] = true
				ordered = append(ordered, id)
				break
			}
		}
	}

	var fields []string
	for _, rec := range records {
		for k := range rec {
			if !seen[k] {
				seen[k] = true
				fields = append(fields, k)
			}
		}
	}
	sort.Strings(fields)
	return append(ordered, fields...)
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

		if _, ok := toFloat(v); !ok {
			allNumber = false
		}
		if _, ok := toBool(v); !ok {
			allBool = false
		}
		if _, ok := toTime(v); !ok {
			allTime = false
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
		// Objects/arrays (multi-relations, multi-selects, file lists, json
		// fields, expanded relations) are serialised as JSON so nested data is
		// still visible.
		b, err := json.Marshal(s)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// timeLayouts lists the formats recognised when inferring time columns. Only
// strings that match one of these (and look date-like) are treated as time.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	// PocketBase autodate/date fields render with a space separator in UTC, e.g.
	// "2022-06-25 11:03:50.052Z" or "2022-06-25 11:03:50Z".
	"2006-01-02 15:04:05.999Z07:00",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05.999",
	"2006-01-02 15:04:05",
	// ISO 8601 variants (in case a text/date field stores them with a `T`).
	"2006-01-02T15:04:05.999Z07:00",
	"2006-01-02T15:04:05Z07:00",
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
