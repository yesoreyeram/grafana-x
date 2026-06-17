package plugin

import (
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

// identityColumns are the Appwrite system attributes promoted to the front of
// each frame, in this order. Every Appwrite document carries them.
var identityColumns = []string{"$id", "$createdAt", "$updatedAt"}

// countToFrame returns a single-row, single-column frame holding a document
// count. It conforms to the data plane "numeric wide" contract so it can be used
// by stat / single-value panels and numeric-aware consumers.
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

// documentsToFrame converts a slice of Appwrite documents (flat JSON maps,
// including the system $-prefixed attributes) into a single wide data.Frame
// conforming to the data plane "table" contract.
//
// Row order is preserved exactly as returned by Appwrite, so the query's sort
// (or the default order) is honoured. Time columns are moved to the front so
// time-series and Explore consumers detect the time dimension, but this does NOT
// change the order of the rows themselves.
//
// When hideSystemFields is true, the Appwrite system columns (the $-prefixed
// fields such as $id, $permissions, $collectionId, $sequence) are dropped from
// the frame. Appwrite always returns these regardless of a `select` query, so
// they are filtered out here.
func documentsToFrame(refID string, documents []map[string]any, hideSystemFields bool) *data.Frame {
	frame := data.NewFrame(refID)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		TypeVersion:            tableTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}

	columns := orderedColumns(documents)
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
		colTypes[col] = inferColumnType(col, documents)
	}
	columns = orderTimeFirst(columns, colTypes)

	rowCount := len(documents)
	fields := make([]*data.Field, len(columns))
	for i, col := range columns {
		fields[i] = buildField(col, documents, rowCount, colTypes[col])
	}
	frame.Fields = fields
	return frame
}

// dropSystemColumns removes the Appwrite system columns (the $-prefixed fields)
// from the column list, preserving the order of the remaining columns.
func dropSystemColumns(columns []string) []string {
	out := make([]string, 0, len(columns))
	for _, col := range columns {
		if strings.HasPrefix(col, "$") {
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

// orderedColumns returns a stable, deterministic column order across documents.
// The Appwrite system identity columns ($id, $createdAt, $updatedAt) lead,
// followed by the union of the remaining attribute names sorted alphabetically.
func orderedColumns(documents []map[string]any) []string {
	seen := map[string]bool{}
	var ordered []string

	// Identity columns first, when present in any document.
	for _, id := range identityColumns {
		for _, doc := range documents {
			if _, ok := doc[id]; ok {
				seen[id] = true
				ordered = append(ordered, id)
				break
			}
		}
	}

	var fields []string
	for _, doc := range documents {
		for k := range doc {
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
func buildField(col string, documents []map[string]any, rowCount int, ft fieldType) *data.Field {
	switch ft {
	case fieldTypeNumber:
		values := make([]*float64, rowCount)
		for i, doc := range documents {
			if f, ok := toFloat(doc[col]); ok {
				v := f
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	case fieldTypeBool:
		values := make([]*bool, rowCount)
		for i, doc := range documents {
			if b, ok := toBool(doc[col]); ok {
				v := b
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	case fieldTypeTime:
		values := make([]*time.Time, rowCount)
		for i, doc := range documents {
			if t, ok := toTime(doc[col]); ok {
				// Normalise to UTC; data plane time fields are timezone-agnostic
				// instants stored as epoch.
				v := t.UTC()
				values[i] = &v
			}
		}
		return data.NewField(col, nil, values)
	default:
		values := make([]*string, rowCount)
		for i, doc := range documents {
			if s, ok := toString(doc[col]); ok {
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
func inferColumnType(col string, documents []map[string]any) fieldType {
	hasValue := false
	allNumber, allBool, allTime := true, true, true

	for _, doc := range documents {
		v, ok := doc[col]
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
		// Objects/arrays (relationships, $permissions, array attributes) are
		// serialised as JSON so nested data is still visible.
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
	// Appwrite datetime attributes and the system $createdAt/$updatedAt render
	// as ISO 8601, e.g. "2026-06-15T09:30:55.000+00:00" or "2026-06-15T09:30:55.000Z".
	"2006-01-02T15:04:05.000Z07:00",
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
