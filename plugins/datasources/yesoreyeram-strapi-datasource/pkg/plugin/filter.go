package plugin

import (
	"fmt"
	"net/url"
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

type SortItem struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// operatorMap maps the editor's logical operator ids to Strapi filter operators.
// Keep this in sync with src/filter.ts.
var operatorMap = map[string]string{
	"eq":          "$eq",
	"ne":          "$ne",
	"gt":          "$gt",
	"gte":         "$gte",
	"lt":          "$lt",
	"lte":         "$lte",
	"contains":    "$contains",
	"containsi":   "$containsi",
	"notContains": "$notContains",
	"startsWith":  "$startsWith",
	"endsWith":    "$endsWith",
	"in":          "$in",
	"notIn":       "$notIn",
	"null":        "$null",
	"notNull":     "$notNull",
}

// BuildFilter compiles a structured filter tree (the root group) into Strapi REST
// filter query parameters.
//
// Strapi filter syntax (URL query params, parsed by the qs library):
//
//	filters[title][$eq]=hello
//	filters[$and][0][title][$eq]=hello&filters[$and][1][views][$gte]=100
//	filters[id][$in][0]=3&filters[id][$in][1]=6
//
// It returns an empty url.Values when there are no valid conditions so the caller
// can omit filters entirely.
func BuildFilter(root *FilterNode) url.Values {
	params := url.Values{}
	if root == nil {
		return params
	}
	compileNode(*root, "filters", params)
	return params
}

func compileNode(node FilterNode, prefix string, params url.Values) {
	if isGroup(node) {
		compileGroup(node, prefix, params)
		return
	}
	compileCondition(node, prefix, params)
}

func compileGroup(group FilterNode, prefix string, params url.Values) {
	valid := make([]FilterNode, 0, len(group.Children))
	for _, child := range group.Children {
		if hasValidConditions(child) {
			valid = append(valid, child)
		}
	}
	switch len(valid) {
	case 0:
		return
	case 1:
		// A single child collapses onto the parent prefix, avoiding a redundant
		// $and/$or wrapper.
		compileNode(valid[0], prefix, params)
	default:
		connector := "$and"
		if strings.EqualFold(group.Connector, "or") {
			connector = "$or"
		}
		for i, child := range valid {
			childPrefix := fmt.Sprintf("%s[%s][%d]", prefix, connector, i)
			compileNode(child, childPrefix, params)
		}
	}
}

func compileCondition(c FilterNode, prefix string, params url.Values) {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "eq"
	}
	strapiOp, ok := operatorMap[op]
	if !ok {
		strapiOp = "$eq"
	}
	base := fmt.Sprintf("%s[%s]", prefix, field)

	switch op {
	case "null", "notNull":
		// Unary operators take a boolean value.
		params.Set(fmt.Sprintf("%s[%s]", base, strapiOp), "true")
	case "in", "notIn":
		// List operators use indexed array syntax: ...[$in][0]=a&...[$in][1]=b.
		for i, v := range parseListValue(c.Value) {
			params.Set(fmt.Sprintf("%s[%s][%d]", base, strapiOp, i), v)
		}
	default:
		params.Set(fmt.Sprintf("%s[%s]", base, strapiOp), c.Value)
	}
}

// isGroup reports whether a node should be treated as a logical group.
func isGroup(node FilterNode) bool {
	if node.Kind == "group" {
		return true
	}
	// Be lenient: a node with children but no field is a group.
	return node.Kind == "" && strings.TrimSpace(node.Field) == "" && len(node.Children) > 0
}

// hasValidConditions reports whether a node contributes at least one usable
// condition, so empty groups/conditions can be skipped without leaving gaps in
// $and/$or index sequences.
func hasValidConditions(node FilterNode) bool {
	if isGroup(node) {
		for _, child := range node.Children {
			if hasValidConditions(child) {
				return true
			}
		}
		return false
	}
	return strings.TrimSpace(node.Field) != ""
}

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
