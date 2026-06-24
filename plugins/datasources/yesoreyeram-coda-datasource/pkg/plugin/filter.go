package plugin

import (
	"encoding/json"
	"strconv"
	"strings"
)

// effectiveFilterQuery returns the value of Coda's rows `query` parameter for a
// query model.
//
// Coda's rows endpoint can only filter by a SINGLE column, using the form
// `<columnIdOrName>:<value>`. There is no general filter language. A raw query
// (q.Query) takes precedence; otherwise a single-column equality filter is built
// from FilterColumn/FilterValue. Anything more complex must be done with Grafana
// transformations.
func effectiveFilterQuery(q QueryModel) string {
	if raw := strings.TrimSpace(q.Query); raw != "" {
		return raw
	}
	return buildColumnQuery(q.FilterColumn, q.FilterValue)
}

// buildColumnQuery builds a single-column equality query of the form
// `<column>:<value>` as understood by Coda's rows endpoint. Column references
// that are not column ids are quoted (required by Coda when using names); values
// are JSON-encoded (numbers/booleans raw, everything else as a quoted string).
func buildColumnQuery(column, value string) string {
	column = strings.TrimSpace(column)
	value = strings.TrimSpace(value)
	if column == "" || value == "" {
		return ""
	}
	return codaColumnRef(column) + ":" + codaQueryValue(value)
}

// codaColumnRef returns a column reference suitable for the `query` parameter.
// Column ids (e.g. "c-tuVwxYz") are used as-is; column names must be quoted.
func codaColumnRef(column string) string {
	if isColumnID(column) {
		return column
	}
	b, _ := json.Marshal(column)
	return string(b)
}

// isColumnID reports whether s looks like a Coda column id (e.g. "c-tuVwxYz").
func isColumnID(s string) bool {
	return strings.HasPrefix(s, "c-")
}

// codaQueryValue JSON-encodes a filter value. Coda treats the value as a JSON
// value, so strings must be quoted while numbers and booleans are emitted raw.
// The column type is unknown at this layer, so numeric-looking and boolean
// values are emitted raw (matching Coda's documented examples).
func codaQueryValue(value string) string {
	switch value {
	case "true", "false":
		return value
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return value
	}
	b, _ := json.Marshal(value)
	return string(b)
}
