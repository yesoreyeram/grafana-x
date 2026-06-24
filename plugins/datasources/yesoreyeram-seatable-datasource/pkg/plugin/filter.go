package plugin

import (
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// The tree is compiled into a *parameterized* SeaTable SQL WHERE clause: every
// user value becomes a `?` placeholder backed by an entry in the parameters
// slice, so values can never break out of the query (no SQL injection). Only
// column and table identifiers are inlined (escaped with backticks).
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields. Field is the SeaTable column *name*.
	Field string `json:"field,omitempty"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// SortItem is a single sort directive: a column name and a direction.
type SortItem struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// BuildWhere converts a structured filter tree (the root group) into a SeaTable
// SQL WHERE fragment plus the ordered parameter values for the `?` placeholders.
// It returns ("", nil) when there are no valid conditions, so the caller can
// omit the WHERE clause entirely.
func BuildWhere(root *FilterNode) (string, []any) {
	if root == nil {
		return "", nil
	}
	return compileGroup(root)
}

func compileNode(node FilterNode) (string, []any) {
	if node.Kind == "group" {
		return compileGroup(&node)
	}
	return compileCondition(node)
}

func compileGroup(group *FilterNode) (string, []any) {
	fragments := make([]string, 0, len(group.Children))
	params := make([]any, 0)
	for i := range group.Children {
		frag, p := compileNode(group.Children[i])
		if frag == "" {
			continue
		}
		fragments = append(fragments, frag)
		params = append(params, p...)
	}
	if len(fragments) == 0 {
		return "", nil
	}
	if len(fragments) == 1 {
		return fragments[0], params
	}
	connector := " AND "
	if strings.EqualFold(group.Connector, "or") {
		connector = " OR "
	}
	// Wrap multi-condition groups in parentheses so AND/OR precedence is
	// preserved when groups are nested.
	return "(" + strings.Join(fragments, connector) + ")", params
}

func compileCondition(c FilterNode) (string, []any) {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return "", nil
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "eq"
	}
	ref := identifier(field)
	value := strings.TrimSpace(c.Value)

	switch op {
	case "empty", "is_empty":
		return ref + " IS NULL", nil
	case "not_empty", "is_not_empty":
		return ref + " IS NOT NULL", nil
	case "is_true":
		// SeaTable SQL supports IS [NOT] TRUE for checkbox columns.
		return ref + " IS TRUE", nil
	case "is_false":
		return ref + " IS NOT TRUE", nil
	case "in", "not_in":
		tokens := splitTokens(c.Value)
		if len(tokens) == 0 {
			return "", nil
		}
		placeholders := make([]string, len(tokens))
		params := make([]any, len(tokens))
		for i, t := range tokens {
			placeholders[i] = "?"
			params[i] = t
		}
		keyword := "IN"
		if op == "not_in" {
			keyword = "NOT IN"
		}
		return ref + " " + keyword + " (" + strings.Join(placeholders, ", ") + ")", params
	case "contains":
		if value == "" {
			return "", nil
		}
		return ref + " ILIKE ?", []any{"%" + value + "%"}
	case "not_contains":
		if value == "" {
			return "", nil
		}
		return "NOT (" + ref + " ILIKE ?)", []any{"%" + value + "%"}
	case "neq":
		if value == "" {
			return "", nil
		}
		return ref + " <> ?", []any{value}
	case "gt":
		return comparison(ref, ">", value)
	case "gte":
		return comparison(ref, ">=", value)
	case "lt":
		return comparison(ref, "<", value)
	case "lte":
		return comparison(ref, "<=", value)
	case "eq":
		fallthrough
	default:
		if value == "" {
			return "", nil
		}
		return ref + " = ?", []any{value}
	}
}

// comparison renders a comparison-operator condition. The value is bound as a
// number when it parses as one (so numeric/date comparisons work), otherwise as
// a string.
func comparison(ref, op, value string) (string, []any) {
	if value == "" {
		return "", nil
	}
	return ref + " " + op + " ?", []any{coerceParam(value)}
}

// coerceParam converts a numeric-looking string into a float64 so SeaTable binds
// it as a number for comparisons; non-numeric values stay strings.
func coerceParam(value string) any {
	if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
		return f
	}
	return value
}

// identifier renders a SeaTable SQL identifier (table/column name) escaped with
// backticks. Embedded backticks are stripped defensively so the identifier can
// never terminate early.
func identifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "") + "`"
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

func splitFields(fields string) []string {
	out := make([]string, 0)
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
