package plugin

import (
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// Attio filters are JSON objects (sent in the records query POST body), not a
// query string. A condition compiles to {"<slug>": {"<$op>": <value>}} and
// groups compile to {"$and": [...]} / {"$or": [...]}. The Category is carried so
// the backend can coerce the editor's string value to the JSON type Attio
// expects (number / boolean / string).
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	Field string `json:"field,omitempty"`
	// Category is the logical attribute category (text, number, boolean, date).
	// It drives value coercion.
	Category string `json:"category,omitempty"`
	Op       string `json:"op,omitempty"`
	Value    string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// SortItem is a single structured sort directive from the query editor.
type SortItem struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// operatorMap maps the editor's operator identifiers to Attio comparison
// operators. Operators that need special handling (neq, empty, not_empty, in)
// are dealt with in compileCondition.
var operatorMap = map[string]string{
	"eq":         "$eq",
	"contains":   "$contains",
	"startsWith": "$starts_with",
	"endsWith":   "$ends_with",
	"gt":         "$gt",
	"gte":        "$gte",
	"lt":         "$lt",
	"lte":        "$lte",
	"in":         "$in",
}

// operatorArity classifies how many values an operator consumes:
//
//	"none"   – unary (no value), e.g. empty / not_empty
//	"single" – a single value (string / number / bool / date)
//	"list"   – comma-separated tokens compiled into an $in array
func operatorArity(op string) string {
	switch op {
	case "empty", "not_empty":
		return "none"
	case "in":
		return "list"
	default:
		return "single"
	}
}

// BuildFilter converts a structured filter tree (the root group) into an Attio
// filter object suitable for the records query body. It returns nil when there
// are no valid conditions, so the caller can omit the filter entirely.
//
// Attio filter format:
//
//	{"slug": {"$eq": "value"}}
//	{"$and": [{"slug": {"$eq": "x"}}, {"slug2": {"$gt": 5}}]}
//
// Attio has no negative operators, so "not equals" and "is empty" are expressed
// by wrapping a positive condition in {"$not": {...}}.
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
	connector := "$and"
	if strings.EqualFold(group.Connector, "or") {
		connector = "$or"
	}
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
		op = "eq"
	}

	switch operatorArity(op) {
	case "none":
		// not_empty -> {"slug": {"$not_empty": true}}
		// empty     -> {"$not": {"slug": {"$not_empty": true}}}
		notEmpty := condition(field, map[string]any{"$not_empty": true})
		if op == "empty" {
			return map[string]any{"$not": notEmpty}
		}
		return notEmpty
	case "list":
		tokens := splitTokens(c.Value)
		if len(tokens) == 0 {
			return nil
		}
		values := make([]any, 0, len(tokens))
		for _, t := range tokens {
			values = append(values, coerceValue(c.Category, t))
		}
		return condition(field, map[string]any{"$in": values})
	default: // single
		v := strings.TrimSpace(c.Value)
		if v == "" {
			return nil
		}
		coerced := coerceValue(c.Category, v)
		// neq has no native operator; negate the $eq condition.
		if op == "neq" {
			return map[string]any{"$not": condition(field, map[string]any{"$eq": coerced})}
		}
		attioOp, ok := operatorMap[op]
		if !ok {
			attioOp = "$eq"
		}
		return condition(field, map[string]any{attioOp: coerced})
	}
}

// condition builds a single Attio attribute filter object.
func condition(slug string, body map[string]any) map[string]any {
	return map[string]any{slug: body}
}

// coerceValue converts the editor's string value into the JSON type Attio
// expects for the given category. Numbers become float64, booleans become bool;
// dates and everything else stay strings (Attio dates are ISO date strings).
func coerceValue(category, value string) any {
	switch category {
	case "number":
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
		return value
	case "boolean":
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
