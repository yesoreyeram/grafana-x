package plugin

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
//
// Depending on its shape the tree is compiled one of two ways:
//   - A single AND group of `eq`/`in` conditions (each column used once) compiles
//     to the Grist records `filter` JSON object (`{"Col":[v1,v2]}`), served by
//     the fast records endpoint.
//   - Anything richer (neq/gt/lt/contains, OR logic, nested groups, repeated
//     columns) compiles to a *parameterized* Grist SQL WHERE clause: every user
//     value becomes a `?` placeholder backed by an entry in the args slice, so
//     values can never break out of the query (no SQL injection). Only column
//     and table identifiers are inlined (escaped with double quotes).
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields. Field is the Grist column id/name.
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

// ---------------------------------------------------------------------------
// Simple membership filter (Grist records endpoint)
// ---------------------------------------------------------------------------

// simpleGristFilter attempts to compile the filter tree into the Grist records
// `filter` JSON object: a mapping of column name -> array of allowed values
// (`{"Col":["a","b"]}`), which the records endpoint matches with membership
// (a row matches if the column equals any listed value; multiple columns are
// AND-ed).
//
// It returns (jsonString, true) when the whole tree is representable this way,
// or ("", false) when the caller must fall back to SQL. A nil/empty tree returns
// ("", true) meaning "no filter".
//
// The tree is representable iff it is a single AND group whose children are all
// `eq`/`in` conditions, with each column used at most once. (Repeated columns,
// OR logic, nested groups and rich operators all require SQL.)
func simpleGristFilter(root *FilterNode) (string, bool) {
	if root == nil {
		return "", true
	}

	filters := map[string][]any{}
	seen := map[string]bool{}
	count := 0

	for _, child := range root.Children {
		if child.Kind == "group" {
			return "", false
		}
		field := strings.TrimSpace(child.Field)
		if field == "" {
			// Skip incomplete conditions (consistent with the SQL compiler).
			continue
		}
		op := strings.TrimSpace(child.Op)
		if op == "" {
			op = "eq"
		}
		if op != "eq" && op != "in" {
			return "", false
		}
		if seen[field] {
			// The same column twice cannot be AND-ed via membership.
			return "", false
		}

		var values []any
		if op == "in" {
			for _, t := range splitTokens(child.Value) {
				values = append(values, t)
			}
		} else if strings.TrimSpace(child.Value) != "" {
			values = []any{child.Value}
		}
		if len(values) == 0 {
			continue
		}

		seen[field] = true
		filters[field] = values
		count++
	}

	// OR across multiple columns is not representable as a membership filter.
	if strings.EqualFold(root.Connector, "or") && count > 1 {
		return "", false
	}
	if count == 0 {
		return "", true
	}

	b, err := json.Marshal(filters)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// ---------------------------------------------------------------------------
// Parameterized SQL WHERE clause (Grist SQL endpoint)
// ---------------------------------------------------------------------------

// BuildWhere converts a structured filter tree (the root group) into a Grist SQL
// WHERE fragment plus the ordered argument values for the `?` placeholders. It
// returns ("", nil) when there are no valid conditions, so the caller can omit
// the WHERE clause entirely.
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
	args := make([]any, 0)
	for i := range group.Children {
		frag, p := compileNode(group.Children[i])
		if frag == "" {
			continue
		}
		fragments = append(fragments, frag)
		args = append(args, p...)
	}
	if len(fragments) == 0 {
		return "", nil
	}
	if len(fragments) == 1 {
		return fragments[0], args
	}
	connector := " AND "
	if strings.EqualFold(group.Connector, "or") {
		connector = " OR "
	}
	// Wrap multi-condition groups in parentheses so AND/OR precedence is
	// preserved when groups are nested.
	return "(" + strings.Join(fragments, connector) + ")", args
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
	case "empty", "is_empty", "blank":
		return ref + " IS NULL", nil
	case "not_empty", "is_not_empty", "not_blank":
		return ref + " IS NOT NULL", nil
	case "in", "not_in":
		tokens := splitTokens(c.Value)
		if len(tokens) == 0 {
			return "", nil
		}
		placeholders := make([]string, len(tokens))
		args := make([]any, len(tokens))
		for i, t := range tokens {
			placeholders[i] = "?"
			// Keep membership values as strings; SQLite applies column affinity
			// so text/numeric columns both match correctly.
			args[i] = t
		}
		keyword := "IN"
		if op == "not_in" {
			keyword = "NOT IN"
		}
		return ref + " " + keyword + " (" + strings.Join(placeholders, ", ") + ")", args
	case "contains":
		if value == "" {
			return "", nil
		}
		return ref + " LIKE ?", []any{"%" + value + "%"}
	case "not_contains":
		if value == "" {
			return "", nil
		}
		return "(" + ref + " NOT LIKE ? OR " + ref + " IS NULL)", []any{"%" + value + "%"}
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
// number when it parses as one (so numeric comparisons work), otherwise as a
// string.
func comparison(ref, op, value string) (string, []any) {
	if value == "" {
		return "", nil
	}
	return ref + " " + op + " ?", []any{coerceArg(value)}
}

// coerceArg converts a numeric-looking string into a float64 so Grist/SQLite
// binds it as a number for comparisons; non-numeric values stay strings.
func coerceArg(value string) any {
	if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
		return f
	}
	return value
}

// identifier renders a Grist SQL identifier (table/column name). Grist documents
// are SQLite databases, where identifiers are quoted with double quotes. Embedded
// double quotes are stripped defensively so the identifier can never terminate
// early.
func identifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, "") + `"`
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

// sortCSV renders the Grist records `sort` parameter from the structured sort
// items: a comma-separated list of column names, each prefixed with `-` for
// descending order (e.g. `Name,-Age`). It returns "" when there are no items.
func sortCSV(items []SortItem) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		if strings.EqualFold(s.Direction, "desc") {
			field = "-" + field
		}
		parts = append(parts, field)
	}
	return strings.Join(parts, ",")
}
