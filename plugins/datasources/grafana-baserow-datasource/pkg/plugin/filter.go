package plugin

import (
	"encoding/json"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
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

// baserowFilter is a single Baserow view filter condition.
type baserowFilter struct {
	Field string `json:"field"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// baserowFilterTree is the JSON shape Baserow expects for the `filters` query
// parameter. See the `filters` parameter on
// GET /api/database/rows/table/{table_id}/. Conditions live in `filters`, nested
// groups live in `groups`; both are combined using `filter_type`.
type baserowFilterTree struct {
	FilterType string              `json:"filter_type"`
	Filters    []baserowFilter     `json:"filters"`
	Groups     []baserowFilterTree `json:"groups,omitempty"`
}

// BuildFilters converts a structured filter tree (the root group) into the JSON
// string Baserow expects for the `filters` query parameter. It returns an empty
// string when there are no valid conditions.
func BuildFilters(root *FilterNode) string {
	if root == nil {
		return ""
	}
	tree, ok := buildGroup(root)
	if !ok {
		return ""
	}
	b, err := json.Marshal(tree)
	if err != nil {
		return ""
	}
	return string(b)
}

// buildGroup converts a group node into a baserowFilterTree. The bool return
// reports whether the group contains any usable condition.
func buildGroup(group *FilterNode) (baserowFilterTree, bool) {
	tree := baserowFilterTree{FilterType: connector(group.Connector)}
	any := false

	for i := range group.Children {
		child := group.Children[i]
		if child.Kind == "group" {
			sub, ok := buildGroup(&child)
			if ok {
				tree.Groups = append(tree.Groups, sub)
				any = true
			}
			continue
		}
		if cond, ok := buildCondition(child); ok {
			tree.Filters = append(tree.Filters, cond)
			any = true
		}
	}

	return tree, any
}

// buildCondition converts a condition node into a baserowFilter. Conditions
// without a field are skipped. Unary operators (empty/not_empty) carry no value.
func buildCondition(c FilterNode) (baserowFilter, bool) {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return baserowFilter{}, false
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "equal"
	}
	value := strings.TrimSpace(c.Value)
	if operatorArity(op) == "none" {
		value = ""
	}
	return baserowFilter{Field: field, Type: op, Value: value}, true
}

// connector normalises a logical connector to the Baserow filter_type value.
func connector(c string) string {
	if strings.EqualFold(c, "or") {
		return "OR"
	}
	return "AND"
}

// operatorArity classifies how many values an operator consumes:
//
//	"none"   – unary (no value), e.g. empty/not_empty/boolean-style toggles
//	"single" – a single value (default)
func operatorArity(op string) string {
	switch op {
	case "empty", "not_empty", "date_equals_today", "date_before_today", "date_after_today":
		return "none"
	default:
		return "single"
	}
}
