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

// recordsToFrame converts a slice of flattened Notion pages (arbitrary JSON
// objects keyed by property name) into a single wide data.Frame conforming to
// the data plane "table" contract.
//
// Row order is preserved exactly as returned by Notion, so the query's `sorts`
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

// timeLayouts lists the formats recognised when inferring time columns. Only
// strings that match one of these (and look date-like) are treated as time.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	// Notion date properties may be date-only or full ISO-8601 timestamps.
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
// Notion page flattening
//
// A Notion page returned by the database query endpoint has deeply-typed
// properties, e.g.:
//
//	{
//	  "id": "...",
//	  "created_time": "2024-01-02T03:04:05.000Z",
//	  "last_edited_time": "...",
//	  "properties": {
//	    "Name":   {"type": "title",       "title":       [{"plain_text": "Alice"}]},
//	    "Active": {"type": "checkbox",    "checkbox":    true},
//	    "MRR":    {"type": "number",      "number":      49.5},
//	    "Tags":   {"type": "multi_select","multi_select":[{"name":"a"},{"name":"b"}]}
//	  }
//	}
//
// flattenPage extracts a scalar (or compact string) per property so the records
// can flow through the same type-inference / frame builder as a flat table. The
// page's own id and timestamp metadata are preserved as columns.
// ---------------------------------------------------------------------------

// notionPage is the minimal page shape consumed from the database query result.
type notionPage struct {
	ID             string                     `json:"id"`
	CreatedTime    string                     `json:"created_time"`
	LastEditedTime string                     `json:"last_edited_time"`
	URL            string                     `json:"url"`
	Properties     map[string]json.RawMessage `json:"properties"`
}

// flattenPages converts raw Notion pages into flat records keyed by property
// name. When fields is non-empty, only those property names are retained (page
// metadata columns are always kept).
func flattenPages(pages []notionPage, fields []string) []map[string]any {
	keep := map[string]bool{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			keep[f] = true
		}
	}
	hasFilter := len(keep) > 0

	out := make([]map[string]any, 0, len(pages))
	for _, page := range pages {
		row := map[string]any{}
		// Page metadata as stable columns.
		if page.CreatedTime != "" {
			row["created_time"] = page.CreatedTime
		}
		if page.LastEditedTime != "" {
			row["last_edited_time"] = page.LastEditedTime
		}
		if page.ID != "" {
			row["id"] = page.ID
		}

		for name, raw := range page.Properties {
			if hasFilter && !keep[name] {
				continue
			}
			row[name] = flattenProperty(raw)
		}
		out = append(out, row)
	}
	return out
}

// flattenProperty reduces a single typed Notion property value to a scalar.
func flattenProperty(raw json.RawMessage) any {
	var prop struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &prop); err != nil {
		return nil
	}

	switch prop.Type {
	case "title", "rich_text":
		var v struct {
			Title    []richText `json:"title"`
			RichText []richText `json:"rich_text"`
		}
		_ = json.Unmarshal(raw, &v)
		items := v.Title
		if prop.Type == "rich_text" {
			items = v.RichText
		}
		return joinRichText(items)
	case "number":
		var v struct {
			Number *float64 `json:"number"`
		}
		_ = json.Unmarshal(raw, &v)
		if v.Number == nil {
			return nil
		}
		return *v.Number
	case "checkbox":
		var v struct {
			Checkbox bool `json:"checkbox"`
		}
		_ = json.Unmarshal(raw, &v)
		return v.Checkbox
	case "select":
		var v struct {
			Select *struct {
				Name string `json:"name"`
			} `json:"select"`
		}
		_ = json.Unmarshal(raw, &v)
		if v.Select == nil {
			return nil
		}
		return v.Select.Name
	case "status":
		var v struct {
			Status *struct {
				Name string `json:"name"`
			} `json:"status"`
		}
		_ = json.Unmarshal(raw, &v)
		if v.Status == nil {
			return nil
		}
		return v.Status.Name
	case "multi_select":
		var v struct {
			MultiSelect []struct {
				Name string `json:"name"`
			} `json:"multi_select"`
		}
		_ = json.Unmarshal(raw, &v)
		names := make([]string, 0, len(v.MultiSelect))
		for _, o := range v.MultiSelect {
			names = append(names, o.Name)
		}
		if len(names) == 0 {
			return nil
		}
		return strings.Join(names, ", ")
	case "date":
		var v struct {
			Date *struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"date"`
		}
		_ = json.Unmarshal(raw, &v)
		if v.Date == nil || v.Date.Start == "" {
			return nil
		}
		return v.Date.Start
	case "created_time":
		var v struct {
			CreatedTime string `json:"created_time"`
		}
		_ = json.Unmarshal(raw, &v)
		return emptyToNil(v.CreatedTime)
	case "last_edited_time":
		var v struct {
			LastEditedTime string `json:"last_edited_time"`
		}
		_ = json.Unmarshal(raw, &v)
		return emptyToNil(v.LastEditedTime)
	case "email":
		return scalarString(raw, "email")
	case "phone_number":
		return scalarString(raw, "phone_number")
	case "url":
		return scalarString(raw, "url")
	case "people":
		var v struct {
			People []struct {
				Name string `json:"name"`
			} `json:"people"`
		}
		_ = json.Unmarshal(raw, &v)
		names := make([]string, 0, len(v.People))
		for _, p := range v.People {
			if p.Name != "" {
				names = append(names, p.Name)
			}
		}
		if len(names) == 0 {
			return nil
		}
		return strings.Join(names, ", ")
	case "files":
		var v struct {
			Files []struct {
				Name string `json:"name"`
			} `json:"files"`
		}
		_ = json.Unmarshal(raw, &v)
		names := make([]string, 0, len(v.Files))
		for _, f := range v.Files {
			names = append(names, f.Name)
		}
		if len(names) == 0 {
			return nil
		}
		return strings.Join(names, ", ")
	case "formula":
		return flattenFormula(raw)
	case "rollup":
		return flattenRollup(raw)
	case "unique_id":
		var v struct {
			UniqueID *struct {
				Prefix string  `json:"prefix"`
				Number float64 `json:"number"`
			} `json:"unique_id"`
		}
		_ = json.Unmarshal(raw, &v)
		if v.UniqueID == nil {
			return nil
		}
		return v.UniqueID.Number
	default:
		// Unknown / complex property types: serialise the type-specific payload
		// so the data is still visible rather than silently dropped.
		var generic map[string]json.RawMessage
		if err := json.Unmarshal(raw, &generic); err == nil {
			if payload, ok := generic[prop.Type]; ok {
				return string(payload)
			}
		}
		return nil
	}
}

func flattenFormula(raw json.RawMessage) any {
	var v struct {
		Formula struct {
			Type    string   `json:"type"`
			String  *string  `json:"string"`
			Number  *float64 `json:"number"`
			Boolean *bool    `json:"boolean"`
			Date    *struct {
				Start string `json:"start"`
			} `json:"date"`
		} `json:"formula"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	switch v.Formula.Type {
	case "string":
		if v.Formula.String == nil {
			return nil
		}
		return *v.Formula.String
	case "number":
		if v.Formula.Number == nil {
			return nil
		}
		return *v.Formula.Number
	case "boolean":
		if v.Formula.Boolean == nil {
			return nil
		}
		return *v.Formula.Boolean
	case "date":
		if v.Formula.Date == nil {
			return nil
		}
		return emptyToNil(v.Formula.Date.Start)
	default:
		return nil
	}
}

func flattenRollup(raw json.RawMessage) any {
	var v struct {
		Rollup struct {
			Type   string   `json:"type"`
			Number *float64 `json:"number"`
			Date   *struct {
				Start string `json:"start"`
			} `json:"date"`
		} `json:"rollup"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	switch v.Rollup.Type {
	case "number":
		if v.Rollup.Number == nil {
			return nil
		}
		return *v.Rollup.Number
	case "date":
		if v.Rollup.Date == nil {
			return nil
		}
		return emptyToNil(v.Rollup.Date.Start)
	default:
		return nil
	}
}

type richText struct {
	PlainText string `json:"plain_text"`
}

func joinRichText(items []richText) any {
	if len(items) == 0 {
		return nil
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.PlainText)
	}
	s := b.String()
	if s == "" {
		return nil
	}
	return s
}

func scalarString(raw json.RawMessage, key string) any {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	payload, ok := generic[key]
	if !ok {
		return nil
	}
	var s *string
	if err := json.Unmarshal(payload, &s); err != nil || s == nil {
		return nil
	}
	return emptyToNil(*s)
}

func emptyToNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
