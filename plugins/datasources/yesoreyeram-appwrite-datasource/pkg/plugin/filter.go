package plugin

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	Attribute string `json:"attribute,omitempty"`
	Op        string `json:"op,omitempty"`
	Value     string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// SortItem is a single sort directive: an attribute key and a direction.
type SortItem struct {
	Attribute string `json:"attribute"`
	Direction string `json:"direction"`
}

// queryString is the on-the-wire representation of a single Appwrite query. The
// SDKs produce exactly this JSON shape and pass it as a `queries[]` parameter.
// See https://appwrite.io/docs/products/databases/queries.
type queryString struct {
	Method    string `json:"method"`
	Attribute string `json:"attribute,omitempty"`
	Values    []any  `json:"values,omitempty"`
}

// encode serialises the query into its JSON string form. It never returns an
// error in practice (the values are always JSON-encodable scalars/slices).
func (q queryString) encode() string {
	b, err := json.Marshal(q)
	if err != nil {
		return ""
	}
	return string(b)
}

// BuildFilterQueries converts a structured filter tree (the root group) into a
// slice of Appwrite query strings. Each top-level child becomes its own query
// string (queries are implicitly AND-ed by Appwrite), except nested groups which
// are compiled into a single `and`/`or` query. It returns nil when there are no
// valid conditions.
func BuildFilterQueries(root *FilterNode) []string {
	if root == nil {
		return nil
	}
	// The root group's connector determines how its direct children combine. When
	// it is the default AND, we can emit each child as a separate query string,
	// which is the idiomatic Appwrite form. For OR we must wrap in a single `or`.
	if strings.EqualFold(root.Connector, "or") {
		if frag, ok := buildGroupQuery(root); ok {
			return []string{frag}
		}
		return nil
	}

	out := make([]string, 0, len(root.Children))
	for i := range root.Children {
		child := root.Children[i]
		if child.Kind == "group" {
			if frag, ok := buildGroupQuery(&child); ok {
				out = append(out, frag)
			}
			continue
		}
		if frag, ok := buildConditionQuery(child); ok {
			out = append(out, frag)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildGroupQuery renders a group node to a single `and`/`or` query string whose
// values are the nested query strings. It returns ok=false when the group has no
// usable conditions.
func buildGroupQuery(group *FilterNode) (string, bool) {
	parts := make([]any, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		if child.Kind == "group" {
			if frag, ok := buildGroupQuery(&child); ok {
				parts = append(parts, frag)
			}
			continue
		}
		if frag, ok := buildConditionQuery(child); ok {
			parts = append(parts, frag)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	if len(parts) == 1 {
		// A single-child group needs no logical wrapper.
		return parts[0].(string), true
	}
	method := "and"
	if strings.EqualFold(group.Connector, "or") {
		method = "or"
	}
	return queryString{Method: method, Values: parts}.encode(), true
}

// buildConditionQuery renders a single condition to an Appwrite query string.
// Conditions without an attribute are skipped (ok=false).
func buildConditionQuery(c FilterNode) (string, bool) {
	attr := strings.TrimSpace(c.Attribute)
	if attr == "" {
		return "", false
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "equal"
	}
	value := c.Value

	switch op {
	case "isNull", "isNotNull":
		// Null-check operators take no value.
		return queryString{Method: op, Attribute: attr}.encode(), true
	case "equal", "notEqual":
		return queryString{Method: op, Attribute: attr, Values: []any{coerce(value)}}.encode(), true
	case "lessThan", "lessThanEqual", "greaterThan", "greaterThanEqual":
		return queryString{Method: op, Attribute: attr, Values: []any{coerce(value)}}.encode(), true
	case "contains", "notContains", "search", "startsWith", "endsWith":
		// String operators always compare against the raw string value.
		return queryString{Method: op, Attribute: attr, Values: []any{value}}.encode(), true
	default:
		// Unknown operator: fall back to equality so the query still runs.
		return queryString{Method: "equal", Attribute: attr, Values: []any{coerce(value)}}.encode(), true
	}
}

// coerce converts a raw string value into the most specific JSON scalar that
// fits (bool, integer, float) so numeric/boolean comparisons work against typed
// Appwrite attributes. Non-numeric, non-boolean values stay strings.
func coerce(value string) any {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}
	return value
}

// orderQueries renders the structured sort items into Appwrite `orderAsc` /
// `orderDesc` query strings, in order.
func orderQueries(items []SortItem) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		attr := strings.TrimSpace(s.Attribute)
		if attr == "" {
			continue
		}
		method := "orderAsc"
		if strings.EqualFold(s.Direction, "desc") {
			method = "orderDesc"
		}
		out = append(out, queryString{Method: method, Attribute: attr}.encode())
	}
	return out
}

// selectQuery renders a `select` query string from a comma-separated list of
// attribute keys, or returns ok=false when the list is empty.
//
// Unless system fields are being hidden, the Appwrite identity attributes
// ($id, $createdAt, $updatedAt) are appended so the frame keeps its identity
// columns even when the user only picked user attributes. (Appwrite always
// returns the remaining system fields regardless of `select`; those are dropped
// frame-side when hideSystemFields is set.)
func selectQuery(attributes string, hideSystemFields bool) (string, bool) {
	keys := make([]any, 0)
	for _, a := range strings.Split(attributes, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			keys = append(keys, a)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	if !hideSystemFields {
		keys = append(keys, "$id", "$createdAt", "$updatedAt")
	}
	return queryString{Method: "select", Values: keys}.encode(), true
}
