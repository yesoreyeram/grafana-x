package plugin

import (
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// Unlike a SQL where clause, PostgREST filters are URL query parameters. A
// top-level AND group compiles to one parameter per condition (`age=gt.18`,
// implicit AND); a top-level OR group compiles to a single `or=(...)` parameter;
// nested groups compile to inline `and(...)`/`or(...)` expressions inside their
// parent's parameter.
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	Field string `json:"field,omitempty"`
	// Op is the operator token. It may carry a `not.` negation prefix
	// (e.g. `not.eq`) and the `is` family carries its target inline
	// (`is.null`, `is.true`, `is.false`, `is.unknown`, `is.not_null`).
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

// Param is a URL query parameter key-value pair.
type Param struct {
	Key   string
	Value string
}

// BuildParams converts a structured filter tree (the root group) into PostgREST
// query parameters. It returns nil when there are no valid conditions, so the
// caller can omit the filter entirely.
//
// See https://postgrest.org/en/stable/references/api/tables_views.html#horizontal-filtering
func BuildParams(root FilterNode) []Param {
	if root.Kind != "group" {
		return nil
	}
	return compileGroupParams(&root)
}

// connectorOf returns the normalised logical connector ("and"/"or") for a group,
// defaulting to "and".
func connectorOf(group *FilterNode) string {
	if strings.EqualFold(group.Connector, "or") {
		return "or"
	}
	return "and"
}

// compileGroupParams compiles a group in a top-level (query-parameter) context.
//
//   - An OR group becomes a single `or=(...)` parameter whose body is the
//     comma-joined inline serialisation of its children.
//   - An AND group becomes one parameter per child: conditions map to
//     `field=op.value`; nested AND groups are flattened into the same context;
//     nested OR groups become their own `or=(...)` parameter.
func compileGroupParams(group *FilterNode) []Param {
	if connectorOf(group) == "or" {
		parts := serializeGroupParts(group)
		if len(parts) == 0 {
			return nil
		}
		return []Param{{Key: "or", Value: "(" + strings.Join(parts, ",") + ")"}}
	}

	params := make([]Param, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		if child.Kind == "group" {
			if connectorOf(&child) == "and" {
				// AND nested in AND: flatten into the same parameter list.
				params = append(params, compileGroupParams(&child)...)
				continue
			}
			parts := serializeGroupParts(&child)
			if len(parts) == 0 {
				continue
			}
			params = append(params, Param{Key: "or", Value: "(" + strings.Join(parts, ",") + ")"})
			continue
		}
		if field, body, ok := conditionBody(child, false); ok {
			params = append(params, Param{Key: field, Value: body})
		}
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

// serializeGroupParts returns the inline serialisation of each valid child of a
// group (used when the group is nested inside another logical operator).
func serializeGroupParts(group *FilterNode) []string {
	parts := make([]string, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		var s string
		if child.Kind == "group" {
			s = serializeInlineGroup(&child)
		} else {
			s = serializeInlineCondition(child)
		}
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

// serializeInlineGroup renders a nested group as an inline `and(...)`/`or(...)`
// expression. A group that reduces to a single condition drops the wrapper.
func serializeInlineGroup(group *FilterNode) string {
	parts := serializeGroupParts(group)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return connectorOf(group) + "(" + strings.Join(parts, ",") + ")"
}

// serializeInlineCondition renders a condition as `field.op.value`, the form used
// inside `or(...)`/`and(...)` groups. Values are quoted when they contain
// reserved characters so the surrounding group is not mis-parsed.
func serializeInlineCondition(c FilterNode) string {
	field, body, ok := conditionBody(c, true)
	if !ok {
		return ""
	}
	return field + "." + body
}

// conditionBody compiles a single condition to its (field, body) pair, where the
// body is the PostgREST operator expression after the column name (e.g.
// `eq.Alice`, `not.gt.18`, `is.null`, `in.(a,b)`). It returns ok=false when the
// condition is incomplete (no field, or a value-requiring operator with no
// value). When inGroup is true, values are quoted if they contain reserved
// characters.
func conditionBody(c FilterNode, inGroup bool) (string, string, bool) {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return "", "", false
	}

	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "eq"
	}
	value := strings.TrimSpace(c.Value)

	// Split an optional leading `not.` negation prefix.
	negated := false
	if strings.HasPrefix(op, "not.") {
		negated = true
		op = strings.TrimPrefix(op, "not.")
	}

	var body string
	switch {
	case op == "is" || strings.HasPrefix(op, "is."):
		// Unary IS check against null/not_null/true/false/unknown.
		target := "null"
		if strings.HasPrefix(op, "is.") {
			target = strings.TrimPrefix(op, "is.")
		} else if value != "" {
			target = strings.ToLower(value)
		}
		switch target {
		case "null", "not_null", "true", "false", "unknown":
		default:
			target = "null"
		}
		body = "is." + target
	case op == "in":
		list := buildInList(value)
		if list == "()" {
			return "", "", false
		}
		body = "in." + list
	default:
		// Binary operator (eq, neq, gt, gte, lt, lte, like, ilike, match,
		// imatch, cs, cd, fts, ...). All require a value.
		if value == "" {
			return "", "", false
		}
		body = op + "." + formatValue(value, inGroup)
	}

	if negated {
		body = "not." + body
	}
	return field, body, true
}

// formatValue renders a scalar value for an operator expression. Inside a
// logical group the value is quoted if it contains reserved characters so the
// group's commas/parentheses are not mis-parsed; at the top level the value is
// passed through unchanged (preserving like/ilike `*`/`%` patterns).
func formatValue(value string, inGroup bool) string {
	if inGroup {
		return quoteIfReserved(value)
	}
	return value
}

// buildInList renders the comma-separated value of an `in` operator into the
// PostgREST list form `(a,b,c)`. Each token is quoted when it contains reserved
// characters. Tokens that themselves need to contain a comma cannot be expressed
// through the comma-separated editor input.
func buildInList(value string) string {
	tokens := make([]string, 0)
	for _, t := range strings.Split(value, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		tokens = append(tokens, quoteIfReserved(t))
	}
	return "(" + strings.Join(tokens, ",") + ")"
}

// quoteIfReserved wraps a value in double quotes when it contains a PostgREST
// reserved character (`,`, `(`, `)`) or already starts with a quote, escaping
// embedded double quotes. Plain values are returned unchanged.
func quoteIfReserved(value string) string {
	if value == "" {
		return value
	}
	if strings.ContainsAny(value, ",()") || strings.HasPrefix(value, `"`) {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
