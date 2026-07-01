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

// recordsToFrame converts a slice of flattened Attio records (arbitrary JSON
// objects keyed by attribute slug) into a single wide data.Frame conforming to
// the data plane "table" contract.
//
// Row order is preserved exactly as returned by Attio, so the query's `sorts`
// is honoured. Time fields are moved to the front of the columns so that
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
	return fields
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

// timeLayouts lists the formats recognised when inferring time columns. Only
// strings that match one of these (and look date-like) are treated as time.
// Attio returns ISO-8601 timestamps (e.g. created_at) and date-only strings
// (date attributes, "YYYY-MM-DD").
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000000000Z07:00",
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

// ---------------------------------------------------------------------------
// Attio record flattening
//
// A record returned by the records query endpoint has deeply-typed attribute
// values. Each attribute slug maps to an ARRAY of historical value objects; the
// first element is the latest active value. Each value object carries an
// `attribute_type` discriminator and a type-specific payload, e.g.:
//
//	{
//	  "id": {"record_id": "...", "object_id": "...", "workspace_id": "..."},
//	  "created_at": "2024-01-02T03:04:05.000000000Z",
//	  "values": {
//	    "name":   [{"attribute_type":"personal-name","full_name":"Ada Lovelace", ...}],
//	    "salary": [{"attribute_type":"currency","currency_value":99,"currency_code":"USD"}],
//	    "active": [{"attribute_type":"checkbox","value":true}]
//	  }
//	}
//
// flattenRecords reduces each attribute to a scalar (or compact string) so the
// records can flow through the same type-inference / frame builder as a flat
// table. Two synthetic columns, `_record_id` and `_created_at`, preserve the
// record's identity and creation time.
// ---------------------------------------------------------------------------

// attioRecord is the minimal record shape consumed from the records query
// result.
type attioRecord struct {
	ID struct {
		WorkspaceID string `json:"workspace_id"`
		ObjectID    string `json:"object_id"`
		RecordID    string `json:"record_id"`
	} `json:"id"`
	CreatedAt string                     `json:"created_at"`
	WebURL    string                     `json:"web_url"`
	Values    map[string]json.RawMessage `json:"values"`
}

// flattenRecords converts raw Attio records into flat maps keyed by attribute
// slug. When fields is non-empty, only those attribute slugs are retained.
// The synthetic identity columns (_record_id, _created_at) are emitted unless
// hideSystem is true.
func flattenRecords(records []attioRecord, fields []string, hideSystem bool) []map[string]any {
	keep := map[string]bool{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			keep[f] = true
		}
	}
	hasFilter := len(keep) > 0

	out := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		row := map[string]any{}
		if !hideSystem {
			if rec.ID.RecordID != "" {
				row["_record_id"] = rec.ID.RecordID
			}
			if rec.CreatedAt != "" {
				row["_created_at"] = rec.CreatedAt
			}
		}

		for slug, raw := range rec.Values {
			if hasFilter && !keep[slug] {
				continue
			}
			row[slug] = flattenValue(raw)
		}
		out = append(out, row)
	}
	return out
}

// flattenValue reduces a single attribute value (an array of historical value
// objects) to a scalar by taking the first (latest active) element and coercing
// it based on its attribute_type discriminator.
func flattenValue(raw json.RawMessage) any {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return nil
	}
	first := arr[0]

	var disc struct {
		AttributeType string `json:"attribute_type"`
	}
	_ = json.Unmarshal(first, &disc)

	switch disc.AttributeType {
	case "text":
		return scalar[string](first, "value")
	case "number", "rating":
		return scalar[float64](first, "value")
	case "checkbox":
		return scalar[bool](first, "value")
	case "date", "timestamp":
		return scalar[string](first, "value")
	case "currency":
		return scalar[float64](first, "currency_value")
	case "select":
		return titleOrID(first, "option")
	case "status":
		return titleOrID(first, "status")
	case "record-reference":
		return scalar[string](first, "target_record_id")
	case "actor-reference":
		if name := scalar[string](first, "name"); name != nil {
			return name
		}
		return scalar[string](first, "referenced_actor_id")
	case "email-address":
		return scalar[string](first, "email_address")
	case "phone-number":
		return scalar[string](first, "phone_number")
	case "domain":
		return scalar[string](first, "domain")
	case "personal-name":
		return scalar[string](first, "full_name")
	case "interaction":
		return scalar[string](first, "interacted_at")
	default:
		// Unknown / complex value types (e.g. location): serialise the value
		// object so the data is still visible rather than silently dropped.
		return string(first)
	}
}

// scalar extracts a single typed field from a value object, returning nil when
// the field is missing or of a different type. The non-nil return is the
// underlying value (not a pointer) so it flows through type inference.
func scalar[T any](raw json.RawMessage, key string) any {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	payload, ok := generic[key]
	if !ok {
		return nil
	}
	var v *T
	if err := json.Unmarshal(payload, &v); err != nil || v == nil {
		return nil
	}
	if s, ok := any(*v).(string); ok && strings.TrimSpace(s) == "" {
		return nil
	}
	return *v
}

// titleOrID resolves a select/status sub-object that may be returned either as a
// bare UUID string or as a nested object carrying a human-readable title.
func titleOrID(raw json.RawMessage, key string) any {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	payload, ok := generic[key]
	if !ok {
		return nil
	}
	// Object form: {"title": "..."} (possibly nested under a status object).
	var obj struct {
		Title  string `json:"title"`
		Status *struct {
			Title string `json:"title"`
		} `json:"status"`
	}
	if json.Unmarshal(payload, &obj) == nil {
		if strings.TrimSpace(obj.Title) != "" {
			return obj.Title
		}
		if obj.Status != nil && strings.TrimSpace(obj.Status.Title) != "" {
			return obj.Status.Title
		}
	}
	// String form: a bare UUID.
	var s string
	if json.Unmarshal(payload, &s) == nil && strings.TrimSpace(s) != "" {
		return s
	}
	return nil
}
