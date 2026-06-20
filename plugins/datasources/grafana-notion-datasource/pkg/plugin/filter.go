package plugin

import (
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// Unlike a SQL-style where clause, Notion filters are JSON objects. Each
// condition compiles to {"property": <name>, <typeKey>: {<op>: <value>}} and
// groups compile to {"and": [...]} / {"or": [...]}. The condition therefore
// needs the property's type category (text/number/checkbox/date/select/...) to
// pick the correct type key and to coerce the value.
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	Field string `json:"field,omitempty"`
	// Category is the logical property category (text, number, checkbox, date,
	// select, multi_select, people, files). It selects the Notion filter type key.
	Category string `json:"category,omitempty"`
	Op       string `json:"op,omitempty"`
	Value    string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// typeKeyForCategory maps a logical property category to the Notion filter
// object key. Notion nests the operator under a key named after the property
// type, e.g. {"property":"Name","rich_text":{"equals":"x"}}.
func typeKeyForCategory(category string) string {
	switch category {
	case "number":
		return "number"
	case "checkbox":
		return "checkbox"
	case "date":
		return "date"
	case "select":
		return "select"
	case "status":
		return "status"
	case "multi_select":
		return "multi_select"
	case "people":
		return "people"
	case "files":
		return "files"
	case "text":
		fallthrough
	default:
		return "rich_text"
	}
}

// operatorArity classifies how many values an operator consumes:
//
//	"none"   – unary (no value), e.g. is_empty/is_not_empty
//	"single" – a single value (string/number/bool/date)
//	"list"   – comma-separated tokens compiled into a nested or-group of
//	           single-value conditions (Notion has no native "in" operator)
func operatorArity(op string) string {
	switch op {
	case "is_empty", "is_not_empty":
		return "none"
	case "in", "not_in":
		return "list"
	default:
		return "single"
	}
}

// BuildFilter converts a structured filter tree (the root group) into a Notion
// filter object suitable for the database query body. It returns nil when there
// are no valid conditions, so the caller can omit the filter entirely.
func BuildFilter(root *FilterNode) map[string]any {
	if root == nil {
		return nil
	}
	return compileGroup(root)
}

func compileNode(node FilterNode) map[string]any {
	if node.Kind == "group" {
		return compileGroup(&node)
	}
	return compileCondition(node)
}

func compileGroup(group *FilterNode) map[string]any {
	parts := make([]map[string]any, 0, len(group.Children))
	for _, child := range group.Children {
		if c := compileNode(child); c != nil {
			parts = append(parts, c)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		return parts[0]
	}
	connector := "and"
	if strings.EqualFold(group.Connector, "or") {
		connector = "or"
	}
	// Notion expects the array under the connector key. Each element may itself
	// be a condition or another and/or group.
	arr := make([]any, len(parts))
	for i, p := range parts {
		arr[i] = p
	}
	return map[string]any{connector: arr}
}

func compileCondition(c FilterNode) map[string]any {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return nil
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = defaultOpForCategory(c.Category)
	}
	typeKey := typeKeyForCategory(c.Category)

	switch operatorArity(op) {
	case "none":
		// Unary operators are encoded as {<op>: true} under the type key.
		return condition(field, typeKey, map[string]any{op: true})
	case "list":
		tokens := splitTokens(c.Value)
		if len(tokens) == 0 {
			return nil
		}
		// Notion has no list membership operator, so expand into a group of
		// single-value conditions. "in" -> OR of equals; "not_in" -> AND of
		// not-equals.
		singleOp, connector := "equals", "or"
		if op == "not_in" {
			singleOp, connector = "does_not_equal", "and"
		}
		parts := make([]any, 0, len(tokens))
		for _, t := range tokens {
			parts = append(parts, condition(field, typeKey, map[string]any{singleOp: coerceValue(c.Category, t)}))
		}
		if len(parts) == 1 {
			return parts[0].(map[string]any)
		}
		return map[string]any{connector: parts}
	default: // single
		v := strings.TrimSpace(c.Value)
		if v == "" {
			return nil
		}
		return condition(field, typeKey, map[string]any{op: coerceValue(c.Category, v)})
	}
}

// condition builds a single Notion property filter object.
func condition(property, typeKey string, body map[string]any) map[string]any {
	return map[string]any{
		"property": property,
		typeKey:    body,
	}
}

// defaultOpForCategory returns a sensible default operator when none is given.
func defaultOpForCategory(category string) string {
	switch category {
	case "checkbox":
		return "equals"
	case "number", "date":
		return "equals"
	case "select", "status":
		return "equals"
	case "multi_select", "people":
		return "contains"
	default: // text
		return "equals"
	}
}

// coerceValue converts the string value from the editor into the JSON type
// Notion expects for the given category. Numbers become float64, checkbox
// becomes bool; everything else stays a string.
func coerceValue(category, value string) any {
	switch category {
	case "number":
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
		return value
	case "checkbox":
		return parseBool(value)
	default:
		return value
	}
}

func parseBool(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "yes", "checked":
		return true
	default:
		return false
	}
}

func splitTokens(value string) []string {
	tokens := make([]string, 0)
	for _, t := range strings.Split(value, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}
