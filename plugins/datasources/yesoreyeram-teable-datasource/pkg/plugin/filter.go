package plugin

import (
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// Teable filters are JSON objects, not query strings. Each condition compiles to
// {"fieldId": <name>, "operator": <op>, "value": <coerced>} and groups compile
// to {"conjunction": "and"|"or", "filterSet": [...]}. The condition needs the
// field's logical category (text/number/boolean/date/select/multiSelect) to
// shape and coerce the value.
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	// Field is the Teable field NAME (filters are sent with fieldKeyType=name).
	Field string `json:"field,omitempty"`
	// Category is the logical field category, used to coerce/shape the value.
	Category string `json:"category,omitempty"`
	// Op is the Teable filter operator (is, isNot, contains, isGreater, ...).
	Op    string `json:"op,omitempty"`
	Value string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// SortItem is a single sort directive: a field name and a direction.
type SortItem struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// filterItem is a single Teable filter condition.
type filterItem struct {
	FieldID  string `json:"fieldId"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

// filterSet is a Teable filter group: {conjunction, filterSet:[item|group, ...]}.
// The top-level filter is always a filterSet.
type filterSet struct {
	Conjunction string `json:"conjunction"`
	FilterSet   []any  `json:"filterSet"`
}

// Operator arity classifications.
const (
	arityNone   = "none"   // unary, no value (isEmpty / isNotEmpty) -> value null
	arityList   = "list"   // value is an array (isAnyOf / hasAnyOf / ...)
	aritySingle = "single" // value is a single scalar / date object
)

// BuildFilter converts a structured filter tree (the root group) into a Teable
// filter object ({conjunction, filterSet}) suitable for the `filter` query
// parameter. It returns nil when there are no valid conditions, so the caller
// can omit the filter entirely.
func BuildFilter(root *FilterNode) *filterSet {
	if root == nil {
		return nil
	}
	return compileGroup(root)
}

func compileGroup(group *FilterNode) *filterSet {
	parts := make([]any, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		if child.Kind == "group" {
			if g := compileGroup(&child); g != nil {
				parts = append(parts, g)
			}
			continue
		}
		if c := compileCondition(child); c != nil {
			parts = append(parts, *c)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	conjunction := "and"
	if strings.EqualFold(group.Connector, "or") {
		conjunction = "or"
	}
	return &filterSet{Conjunction: conjunction, FilterSet: parts}
}

func compileCondition(c FilterNode) *filterItem {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return nil
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = defaultOperator(c.Category)
	}

	switch operatorArity(op) {
	case arityNone:
		// Unary operators must carry a null value per the Teable schema.
		return &filterItem{FieldID: field, Operator: op, Value: nil}
	case arityList:
		values := splitList(c.Value)
		if len(values) == 0 {
			return nil
		}
		return &filterItem{FieldID: field, Operator: op, Value: values}
	default: // single
		v := strings.TrimSpace(c.Value)
		if v == "" {
			return nil
		}
		return &filterItem{FieldID: field, Operator: op, Value: coerceValue(c.Category, v)}
	}
}

// operatorArity classifies how many values a Teable operator consumes.
func operatorArity(op string) string {
	switch op {
	case "isEmpty", "isNotEmpty":
		return arityNone
	case "isAnyOf", "isNoneOf", "hasAnyOf", "hasAllOf", "hasNoneOf", "isExactly", "isNotExactly":
		return arityList
	default:
		return aritySingle
	}
}

// defaultOperator returns a sensible default operator when none is given.
func defaultOperator(category string) string {
	if category == "multiSelect" {
		return "hasAnyOf"
	}
	return "is"
}

// coerceValue converts the string value from the editor into the JSON type
// Teable expects for the given category. Numbers become float64, checkbox
// becomes bool, dates become a {mode:exactDate,...} object; everything else
// stays a string.
func coerceValue(category, value string) any {
	switch category {
	case "number":
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
		return value
	case "boolean":
		return parseBool(value)
	case "date":
		// Teable date operators expect a structured value with a mode and a
		// timezone. exactDate compares against a specific ISO date.
		return map[string]any{
			"mode":      "exactDate",
			"exactDate": value,
			"timeZone":  "UTC",
		}
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

// splitList splits a comma-separated value into trimmed, non-empty tokens for
// list operators (isAnyOf, hasAnyOf, ...).
func splitList(value string) []string {
	out := make([]string, 0)
	for _, t := range strings.Split(value, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
