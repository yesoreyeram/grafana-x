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

// recordsToFrame converts a slice of flattened monday.com records (arbitrary
// JSON objects keyed by field name) into a single wide data.Frame conforming to
// the data plane "table" contract.
//
// Row order is preserved exactly as returned by monday.com, so the query's
// ordering is honoured. Time fields are moved to the front of the columns so that
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
		// Objects/arrays are serialised as JSON so nested data is still visible.
		b, err := json.Marshal(s)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// timeLayouts lists the formats recognised when inferring time columns. monday
// returns timestamps like "2024-01-02T03:04:05Z" / "2024-01-02 03:04:05 UTC"
// and date-only strings.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000-07:00",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02 15:04:05 MST",
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
// monday.com node flattening
//
// monday.com GraphQL nodes are nested JSON objects. Items additionally carry a
// `column_values` array of {id, column{title}, text, value} objects, e.g.:
//
//	{
//	  "id": "123",
//	  "name": "Task A",
//	  "state": "active",
//	  "created_at": "2024-01-02T03:04:05Z",
//	  "group": {"id": "g1", "title": "Doing"},
//	  "board": {"id": "9", "name": "Tasks"},
//	  "column_values": [
//	    {"id": "status", "column": {"title": "Status"}, "text": "Working", "value": "..."},
//	    {"id": "person", "column": {"title": "Owner"}, "text": "Alice", "value": "..."}
//	  ]
//	}
//
// flattenItem reduces each nested relation to a scalar and lifts every column
// value into a top-level column keyed by its column title (falling back to its
// id), using the human-readable `text` representation.
// ---------------------------------------------------------------------------

// systemColumnTypes are monday.com's built-in/auto-generated column types. These
// are added by monday rather than created by the user; hideSystem omits them.
var systemColumnTypes = map[string]bool{
	"subtasks":     true, // subitems column
	"subitems":     true,
	"last_updated": true,
	"creation_log": true,
	"item_id":      true,
	"auto_number":  true,
	"button":       true,
	"progress":     true, // progress tracking (battery) is derived
	"formula":      true,
}

// isSystemColumn reports whether a column (by type or id) is a monday system
// column that should be hidden when HideSystemColumns is set.
func isSystemColumn(colType, colID string) bool {
	if systemColumnTypes[colType] {
		return true
	}
	// monday's built-in columns use these well-known ids.
	switch colID {
	case "subitems", "subtasks", "name", "person", "creation_log", "last_updated":
		// "name" and "person" are usually user-facing; only treat the clearly
		// internal ones as system here.
	}
	return false
}

// flattenItem flattens a monday.com item node, including its column values when
// requested. When hideSystem is true, monday's built-in/system column values are
// omitted.
func flattenItem(raw json.RawMessage, withColumns, hideSystem bool) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	row := make(map[string]any, len(obj))
	// Process the core item fields first so that column-value collisions (e.g. a
	// column literally titled "name") are detected and renamed deterministically,
	// regardless of Go's map iteration order.
	for key, value := range obj {
		if key == "column_values" {
			continue
		}
		row[key] = flattenValue(value)
	}
	if withColumns {
		if cv, ok := obj["column_values"]; ok {
			flattenColumnValues(cv, row, hideSystem)
		}
	}
	return row
}

// flattenColumnValues lifts each column value into the row, keyed by the column
// title (falling back to the column id). Values are converted using the column
// `type`: checkbox columns become booleans; everything else uses the
// human-readable `text` (a blank/null text leaves the cell empty). When
// hideSystem is true, monday system columns are skipped.
func flattenColumnValues(raw json.RawMessage, row map[string]any, hideSystem bool) {
	var cols []struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Column struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"column"`
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(raw, &cols); err != nil {
		return
	}
	for _, col := range cols {
		colID := col.ID
		if colID == "" {
			colID = col.Column.ID
		}
		if hideSystem && isSystemColumn(col.Type, colID) {
			continue
		}

		key := strings.TrimSpace(col.Column.Title)
		if key == "" {
			key = colID
		}
		if key == "" {
			continue
		}
		// Avoid clobbering core item fields (e.g. a column literally named "name").
		if _, exists := row[key]; exists {
			key = key + " (column)"
		}

		// Checkbox columns are booleans: monday returns text "v" when checked and
		// an empty string when unchecked.
		if col.Type == "checkbox" || col.Type == "check" {
			row[key] = col.Text != nil && strings.TrimSpace(*col.Text) != ""
			continue
		}

		if col.Text == nil || *col.Text == "" {
			row[key] = nil
			continue
		}
		row[key] = *col.Text
	}
}

// flattenNode converts a single monday.com node object into a flat record keyed
// by field name. Nested relations are reduced to readable scalars. This is the
// generic flattener used for boards/users/workspaces/groups/tags and raw nodes.
func flattenNode(raw json.RawMessage) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	row := make(map[string]any, len(obj))
	for key, value := range obj {
		row[key] = flattenValue(value)
	}
	return row
}

// flattenValue reduces a single JSON value to a scalar suitable for a data frame
// cell. Scalars pass through; nested objects are reduced to a representative
// field; arrays of named objects are joined.
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
		// Scalar: number, bool, or string.
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

// flattenObject reduces a nested object to a representative scalar using the
// first available of name/title/text/email/id.
func flattenObject(raw json.RawMessage) any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"name", "title", "text", "email", "id"} {
		if v, ok := obj[key]; ok {
			if s, ok := stringScalar(v); ok {
				return s
			}
		}
	}
	// Unknown object shape: serialise so data is still visible.
	return string(raw)
}

// flattenArray joins an array of named objects (or scalars) into a single
// comma-separated string.
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
