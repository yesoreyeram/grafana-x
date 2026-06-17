package plugin

import (
	"fmt"
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

// SortItem is a single sort directive: a field name and a direction.
type SortItem struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// BuildFormula converts a structured filter tree (the root group) into an
// Airtable `filterByFormula` expression. It returns an empty string when there
// are no valid conditions.
//
// See https://support.airtable.com/docs/formula-field-reference for the formula
// language. Conditions reference fields with the `{Field Name}` syntax; logical
// groups use AND(...) / OR(...).
func BuildFormula(root *FilterNode) string {
	if root == nil {
		return ""
	}
	return buildGroupFormula(root)
}

// buildGroupFormula renders a group node to a formula fragment, combining its
// non-empty children with AND(...) or OR(...). It returns "" when the group has
// no usable conditions.
func buildGroupFormula(group *FilterNode) string {
	parts := make([]string, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		var frag string
		if child.Kind == "group" {
			frag = buildGroupFormula(&child)
		} else {
			frag = buildConditionFormula(child)
		}
		if frag != "" {
			parts = append(parts, frag)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	fn := "AND"
	if strings.EqualFold(group.Connector, "or") {
		fn = "OR"
	}
	return fn + "(" + strings.Join(parts, ", ") + ")"
}

// buildConditionFormula renders a single condition to a formula fragment.
// Conditions without a field are skipped (return "").
func buildConditionFormula(c FilterNode) string {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return ""
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "eq"
	}
	ref := fieldRef(field)
	value := strings.TrimSpace(c.Value)

	switch op {
	case "empty":
		// Treat blank string as empty (covers text/number/blank cells).
		return fmt.Sprintf("{%s} = BLANK()", escapeFieldName(field))
	case "not_empty":
		return fmt.Sprintf("NOT({%s} = BLANK())", escapeFieldName(field))
	case "eq":
		return fmt.Sprintf("%s = %s", ref, quote(value))
	case "neq":
		return fmt.Sprintf("%s != %s", ref, quote(value))
	case "gt":
		return fmt.Sprintf("%s > %s", ref, numberOrQuote(value))
	case "gte":
		return fmt.Sprintf("%s >= %s", ref, numberOrQuote(value))
	case "lt":
		return fmt.Sprintf("%s < %s", ref, numberOrQuote(value))
	case "lte":
		return fmt.Sprintf("%s <= %s", ref, numberOrQuote(value))
	case "contains":
		// FIND returns 0 when not found; > 0 means the substring is present.
		return fmt.Sprintf("FIND(LOWER(%s), LOWER(%s & \"\")) > 0", quote(value), ref)
	case "not_contains":
		return fmt.Sprintf("FIND(LOWER(%s), LOWER(%s & \"\")) = 0", quote(value), ref)
	case "is_true":
		return ref
	case "is_false":
		return fmt.Sprintf("NOT(%s)", ref)
	default:
		// Unknown operator: fall back to equality so the query still runs.
		return fmt.Sprintf("%s = %s", ref, quote(value))
	}
}

// fieldRef returns the `{Field Name}` reference for use in a formula.
func fieldRef(field string) string {
	return "{" + escapeFieldName(field) + "}"
}

// escapeFieldName escapes characters that would break the `{...}` field
// reference. Airtable does not allow `}` inside a field reference; strip it
// defensively.
func escapeFieldName(field string) string {
	return strings.ReplaceAll(field, "}", "")
}

// quote renders a string literal for a formula, escaping embedded double quotes.
func quote(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

// numberOrQuote renders the value as a bare number when it parses as one,
// otherwise as a quoted string literal. Comparison operators work on numbers and
// dates; quoting non-numeric values keeps the formula valid.
func numberOrQuote(value string) string {
	if isNumeric(value) {
		return value
	}
	return quote(value)
}

// isNumeric reports whether s looks like a plain decimal number.
func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	seenDot := false
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '-' && i == 0:
		case r == '.' && !seenDot:
			seenDot = true
		default:
			return false
		}
	}
	return s != "-" && s != "." && s != "-."
}
