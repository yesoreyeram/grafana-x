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

// recordsToFrame converts a slice of flattened Confluence records (arbitrary
// JSON objects keyed by column name) into a single wide data.Frame conforming to
// the data plane "table" contract.
//
// Row order is preserved exactly as returned by Confluence so the query's sort
// (or CQL ordering) is honoured. Time fields are moved to the front of the
// columns so that time-series and Explore consumers detect the time dimension,
// but this does NOT change the order of the rows themselves. When fields is
// non-empty, only those columns are emitted.
func recordsToFrame(refID string, records []map[string]any, fields []string) *data.Frame {
	frame := data.NewFrame(refID)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{
		Type:                   data.FrameTypeTable,
		TypeVersion:            tableTypeVersion,
		PreferredVisualization: data.VisTypeTable,
	}

	columns := selectColumns(orderedColumns(records), fields)
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

// selectColumns keeps only the requested columns (preserving their existing
// order) when fields is non-empty; otherwise returns all columns.
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
		if _, ok := toTimeValue(v); !ok {
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
// Confluence emits ISO-8601 timestamps (createdAt, version.createdAt) with
// milliseconds and a Z suffix.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05.000-07:00",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// toTimeValue parses a value into a time only when it is a string. Numeric
// values are never treated as time (Confluence ids are numeric strings, never
// timestamps).
func toTimeValue(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	return toTime(s)
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
// Confluence content flattening
//
// A page/blog post returned by the v2 listing endpoints looks like:
//
//	{
//	  "id": "123",
//	  "status": "current",
//	  "title": "Release notes",
//	  "spaceId": "456",
//	  "authorId": "557058:...",
//	  "createdAt": "2024-01-02T03:04:05.000Z",
//	  "version": { "number": 3, "message": "edit", "createdAt": "...", "authorId": "..." },
//	  "_links": { "webui": "/spaces/ENG/pages/123/Release+notes" }
//	}
//
// flattenContentItems reduces each item to a flat record of scalar columns so it
// can flow through the shared type-inference / frame builder.
// ---------------------------------------------------------------------------

type contentItem struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	SpaceID   string `json:"spaceId"`
	ParentID  string `json:"parentId"`
	AuthorID  string `json:"authorId"`
	OwnerID   string `json:"ownerId"`
	CreatedAt string `json:"createdAt"`
	Version   *struct {
		Number    *float64 `json:"number"`
		Message   string   `json:"message"`
		CreatedAt string   `json:"createdAt"`
		AuthorID  string   `json:"authorId"`
		MinorEdit *bool    `json:"minorEdit"`
	} `json:"version"`
	Links struct {
		WebUI  string `json:"webui"`
		EditUI string `json:"editui"`
		TinyUI string `json:"tinyui"`
	} `json:"_links"`
}

// flattenContentItems converts raw page/blog post items into flat records. The
// origin is prepended to the relative `_links.webui` so the link is clickable.
func flattenContentItems(items []json.RawMessage, origin string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		var item contentItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		row := map[string]any{
			"id":               emptyToNil(item.ID),
			"title":            emptyToNil(item.Title),
			"spaceId":          emptyToNil(item.SpaceID),
			"status":           emptyToNil(item.Status),
			"authorId":         emptyToNil(item.AuthorID),
			"createdAt":        emptyToNil(item.CreatedAt),
			"versionNumber":    nil,
			"versionMessage":   nil,
			"versionCreatedAt": nil,
			"webui":            absoluteLink(origin, item.Links.WebUI),
		}
		if item.Version != nil {
			if item.Version.Number != nil {
				row["versionNumber"] = *item.Version.Number
			}
			row["versionMessage"] = emptyToNil(item.Version.Message)
			row["versionCreatedAt"] = emptyToNil(item.Version.CreatedAt)
		}
		out = append(out, row)
	}
	return out
}

// ---------------------------------------------------------------------------
// CQL search result flattening
//
// The v1 search endpoint returns results wrapped in a `content` object plus
// search-specific fields:
//
//	{
//	  "content": { "id": "123", "type": "page", "status": "current", "title": "..." },
//	  "title": "Release @@@hl@@@notes@@@endhl@@@",
//	  "excerpt": "...",
//	  "url": "/spaces/ENG/pages/123",
//	  "entityType": "content",
//	  "lastModified": "2024-01-02T03:04:05.000Z"
//	}
// ---------------------------------------------------------------------------

type searchItem struct {
	Content *struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Title   string `json:"title"`
		SpaceID string `json:"spaceId"`
	} `json:"content"`
	Title        string `json:"title"`
	Excerpt      string `json:"excerpt"`
	URL          string `json:"url"`
	EntityType   string `json:"entityType"`
	LastModified string `json:"lastModified"`
}

func flattenSearchItems(items []json.RawMessage, origin string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		var item searchItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		row := map[string]any{
			"id":           nil,
			"type":         nil,
			"status":       nil,
			"spaceId":      nil,
			"title":        nil,
			"excerpt":      emptyToNil(stripHighlight(item.Excerpt)),
			"url":          absoluteLink(origin, item.URL),
			"lastModified": emptyToNil(item.LastModified),
		}
		title := item.Title
		if item.Content != nil {
			row["id"] = emptyToNil(item.Content.ID)
			row["type"] = emptyToNil(item.Content.Type)
			row["status"] = emptyToNil(item.Content.Status)
			row["spaceId"] = emptyToNil(item.Content.SpaceID)
			if item.Content.Title != "" {
				title = item.Content.Title
			}
		}
		row["title"] = emptyToNil(stripHighlight(title))
		out = append(out, row)
	}
	return out
}

// stripHighlight removes the @@@hl@@@ / @@@endhl@@@ search highlight markers that
// Confluence inserts into search result titles and excerpts.
func stripHighlight(s string) string {
	s = strings.ReplaceAll(s, "@@@hl@@@", "")
	s = strings.ReplaceAll(s, "@@@endhl@@@", "")
	return strings.TrimSpace(s)
}

// absoluteLink prepends the origin to a relative link. It returns nil when the
// link is empty so the column is nullable.
func absoluteLink(origin, link string) any {
	link = strings.TrimSpace(link)
	if link == "" {
		return nil
	}
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	if origin == "" {
		return link
	}
	if strings.HasPrefix(link, "/") {
		return origin + link
	}
	return origin + "/" + link
}

func emptyToNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
