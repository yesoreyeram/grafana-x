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

// recordsToFrame converts a slice of flattened Asana records (arbitrary JSON
// objects keyed by field name) into a single wide data.Frame conforming to the
// data plane "table" contract.
//
// Row order is preserved exactly as returned by Asana, so the API's ordering is
// honoured. Time fields are moved to the front of the columns so that
// time-series and Explore consumers detect the time dimension, but this does NOT
// change the order of the rows themselves.
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

	// Determine the type of each column once, then order time columns first so
	// the frame presents its time dimension up front.
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

// selectColumns restricts the column set to the requested fields (in their
// sorted order), preserving only those that actually exist. An empty/nil
// selection returns all columns unchanged.
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
			if t, ok := toColumnTime(rec[col]); ok {
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
// specific type that fits every value. Asana date fields are ISO-8601 strings
// (e.g. "2012-02-22T02:06:58.158Z" or "2024-01-01"); a column whose every value
// parses as such a date is treated as time. Mixed columns fall back to string.
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

// toColumnTime interprets a string value as an ISO/RFC3339 (or date-only) time.
// Non-string values are never treated as time, so numeric ids and counts stay
// numeric.
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
		// Objects/arrays are serialised as JSON so nested data is still visible.
		b, err := json.Marshal(s)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// timeLayouts lists the formats recognised when inferring time columns from
// Asana date strings (ISO-8601 timestamps and date-only values).
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

// ---------------------------------------------------------------------------
// Asana entity flattening
//
// An Asana resource (task, project, ...) is a nested JSON object. flattenItem
// reduces each nested object/array to a scalar (or compact string) so records
// can flow through the same type-inference / frame builder as a flat table.
// Known relations are mapped to their most useful representation; other nested
// values fall back to a generic flattener that picks an object's name/title/id
// or joins an array of named objects.
// ---------------------------------------------------------------------------

// flattenItem converts a single Asana resource object into a flat record keyed
// by field name. Nested relations are reduced to readable scalars.
func flattenItem(raw json.RawMessage) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	row := make(map[string]any, len(obj))
	// custom_fields is expanded into one column per field; defer it until the
	// standard columns are populated so collisions can be detected.
	var customFields json.RawMessage
	for key, value := range obj {
		switch key {
		case "custom_fields":
			customFields = value
		case "assignee", "parent", "workspace", "owner", "team", "created_by", "completed_by":
			// e.g. assignee: {"gid":"1","name":"Alice"} -> "Alice".
			row[key] = pickField(value, "name")
		case "current_status":
			// project current_status: {"text":"On track",...} -> "On track".
			row[key] = pickField(value, "text")
		case "projects", "tags", "followers", "members":
			// arrays of named objects -> "A, B, C".
			row[key] = joinNames(value, "name")
		default:
			row[key] = flattenValue(value)
		}
	}
	if len(customFields) > 0 {
		addCustomFields(customFields, row)
	}
	return row
}

// namedValue is the compact shape used for enum / multi_enum / people custom
// field values (and other Asana named objects).
type namedValue struct {
	Name string `json:"name"`
}

// asanaCustomField is the subset of an Asana custom field value object needed to
// reduce it to a single scalar column value.
type asanaCustomField struct {
	Name            string       `json:"name"`
	Type            string       `json:"type"`
	DisplayValue    *string      `json:"display_value"`
	TextValue       *string      `json:"text_value"`
	NumberValue     *float64     `json:"number_value"`
	EnumValue       *namedValue  `json:"enum_value"`
	MultiEnumValues []namedValue `json:"multi_enum_values"`
	PeopleValue     []namedValue `json:"people_value"`
	DateValue       *struct {
		Date     *string `json:"date"`
		DateTime *string `json:"date_time"`
	} `json:"date_value"`
}

// value reduces a custom field to a single typed scalar: the typed value where
// available (number/text/enum/multi_enum/date/people), falling back to Asana's
// computed display_value. Returns nil when the field has no value.
func (cf asanaCustomField) value() any {
	switch cf.Type {
	case "number":
		if cf.NumberValue != nil {
			return *cf.NumberValue
		}
	case "text":
		if cf.TextValue != nil && *cf.TextValue != "" {
			return *cf.TextValue
		}
	case "enum":
		if cf.EnumValue != nil && cf.EnumValue.Name != "" {
			return cf.EnumValue.Name
		}
	case "multi_enum":
		if s := joinNamedValues(cf.MultiEnumValues); s != "" {
			return s
		}
	case "people":
		if s := joinNamedValues(cf.PeopleValue); s != "" {
			return s
		}
	case "date":
		if cf.DateValue != nil {
			if cf.DateValue.DateTime != nil && *cf.DateValue.DateTime != "" {
				return *cf.DateValue.DateTime
			}
			if cf.DateValue.Date != nil && *cf.DateValue.Date != "" {
				return *cf.DateValue.Date
			}
		}
	}
	// Fallback to Asana's computed display string for any type without a typed
	// value above (e.g. unknown/future field types).
	if cf.DisplayValue != nil && *cf.DisplayValue != "" {
		return *cf.DisplayValue
	}
	return nil
}

// addCustomFields expands an Asana custom_fields array into one column per custom
// field, keyed by the field name. Fields with no value become null columns so
// the column set stays consistent across rows in the same project.
func addCustomFields(raw json.RawMessage, row map[string]any) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return
	}
	for _, item := range items {
		var cf asanaCustomField
		if err := json.Unmarshal(item, &cf); err != nil {
			continue
		}
		name := strings.TrimSpace(cf.Name)
		if name == "" {
			continue
		}
		row[customFieldKey(name, row)] = cf.value()
	}
}

// customFieldKey returns a column key for a custom field, suffixing on collision
// so a custom field never clobbers a standard column (or another custom field of
// the same name).
func customFieldKey(name string, row map[string]any) string {
	if _, exists := row[name]; !exists {
		return name
	}
	for i := 2; ; i++ {
		k := name + " (" + strconv.Itoa(i) + ")"
		if _, exists := row[k]; !exists {
			return k
		}
	}
}

// joinNamedValues joins the names of an array of named values into "A, B, C".
func joinNamedValues(items []namedValue) string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		if n := strings.TrimSpace(it.Name); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}

// pickField returns the named field's scalar value from a JSON object, or nil.
func pickField(raw json.RawMessage, field string) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	if v, ok := obj[field]; ok {
		return flattenValue(v)
	}
	return nil
}

// joinNames joins an array of objects into a comma-separated string of their
// named field (e.g. each project's "name").
func joinNames(raw json.RawMessage, field string) any {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		if v, ok := item[field]; ok {
			if s, ok := stringScalar(v); ok {
				names = append(names, s)
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return strings.Join(names, ", ")
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

// flattenObject reduces a nested object to a representative scalar, using the
// first available of name/title/text/username/id.
func flattenObject(raw json.RawMessage) any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"name", "title", "text", "username", "id", "gid"} {
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
