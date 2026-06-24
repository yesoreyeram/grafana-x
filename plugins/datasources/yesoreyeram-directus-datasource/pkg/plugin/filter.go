package plugin

import (
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// Conditions compile to {"field": {"_op": value}}; groups compile to
// {"_and": [...]} / {"_or": [...]}. See
// https://docs.directus.io/guides/connect/filter-rules
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	Field string `json:"field,omitempty"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// SortItem is a single structured sort directive persisted by the query editor.
type SortItem struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// operatorMap maps the query editor's operator keys to Directus filter
// operators. The `not_empty` key is retained as a backward-compatible alias for
// previously-saved queries.
var operatorMap = map[string]string{
	"eq":          "_eq",
	"neq":         "_neq",
	"lt":          "_lt",
	"lte":         "_lte",
	"gt":          "_gt",
	"gte":         "_gte",
	"contains":    "_contains",
	"ncontains":   "_ncontains",
	"icontains":   "_icontains",
	"nicontains":  "_nicontains",
	"startsWith":  "_starts_with",
	"nstartsWith": "_nstarts_with",
	"endsWith":    "_ends_with",
	"nendsWith":   "_nends_with",
	"in":          "_in",
	"nin":         "_nin",
	"between":     "_between",
	"nbetween":    "_nbetween",
	"null":        "_null",
	"nnull":       "_nnull",
	"empty":       "_empty",
	"nempty":      "_nempty",
	"not_empty":   "_nempty",
}

// operatorArity classifies how an operator's value is interpreted:
//
//	"none"    – unary (encoded as boolean true), e.g. _null/_empty
//	"list"    – comma-separated tokens compiled into a JSON array, e.g. _in
//	"between" – exactly two comma-separated tokens compiled into a [min,max] array
//	"single"  – a single scalar value (default)
func operatorArity(op string) string {
	switch op {
	case "null", "nnull", "empty", "nempty", "not_empty":
		return "none"
	case "in", "nin":
		return "list"
	case "between", "nbetween":
		return "between"
	default:
		return "single"
	}
}

// BuildFilter converts a structured filter tree (the root group) into a Directus
// JSON filter object. It returns nil when there are no valid conditions, so the
// caller can omit the filter entirely.
//
// Directus filter format:
//
//	{"field_name": {"_eq": "value"}}
//	{"_and": [{"field": {"_eq": "val1"}}, {"field2": {"_neq": "val2"}}]}
func BuildFilter(root *FilterNode) map[string]any {
	if root == nil {
		return nil
	}
	return buildGroupFilter(root)
}

func buildGroupFilter(group *FilterNode) map[string]any {
	parts := make([]map[string]any, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		var frag map[string]any
		if child.Kind == "group" {
			frag = buildGroupFilter(&child)
		} else {
			frag = buildConditionFilter(child)
		}
		if frag != nil {
			parts = append(parts, frag)
		}
	}

	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		return parts[0]
	}

	key := "_and"
	if strings.EqualFold(group.Connector, "or") {
		key = "_or"
	}
	return map[string]any{key: parts}
}

func buildConditionFilter(c FilterNode) map[string]any {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return nil
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "eq"
	}

	directusOp, ok := operatorMap[op]
	if !ok {
		directusOp = "_eq"
	}

	var value any
	switch operatorArity(op) {
	case "none":
		// Directus expects a boolean true for null/empty checks.
		value = true
	case "list":
		tokens := parseListValue(c.Value)
		if len(tokens) == 0 {
			return nil
		}
		value = tokens
	case "between":
		tokens := parseListValue(c.Value)
		if len(tokens) != 2 {
			// _between requires exactly two values; drop incomplete ranges.
			return nil
		}
		value = tokens
	default: // single
		v := strings.TrimSpace(c.Value)
		if v == "" {
			// Drop incomplete conditions so an empty value is not sent as a
			// filter for the empty string.
			return nil
		}
		value = v
	}

	return map[string]any{
		field: map[string]any{
			directusOp: value,
		},
	}
}

// parseListValue splits a comma-separated value into trimmed, non-empty tokens.
func parseListValue(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
