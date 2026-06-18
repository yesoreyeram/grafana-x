package plugin

import (
	"strconv"
	"strings"
)

// FilterNode is a node in the structured filter tree sent from the query editor.
// A node is either a condition (kind="condition") or a group (kind="group").
type FilterNode struct {
	Kind string `json:"kind"`

	// Condition fields.
	Attribute string `json:"attribute,omitempty"`
	Op        string `json:"op,omitempty"`
	Value     string `json:"value,omitempty"`

	// Group fields.
	Connector string       `json:"connector,omitempty"`
	Children  []FilterNode `json:"children,omitempty"`
}

// SortItem is a single sort directive: a field name and a direction.
type SortItem struct {
	Attribute string `json:"attribute"`
	Direction string `json:"direction"`
}

// operatorSymbols maps the query editor operator names to PocketBase filter
// comparison operators. See https://pocketbase.io/docs/api-records/#listsearch-records
var operatorSymbols = map[string]string{
	"equal":            "=",
	"notEqual":         "!=",
	"greaterThan":      ">",
	"greaterThanEqual": ">=",
	"lessThan":         "<",
	"lessThanEqual":    "<=",
	"contains":         "~",
	"notContains":      "!~",
}

// textMatchOps are operators whose right-hand operand is always treated as a
// string literal (PocketBase auto-wraps `~`/`!~` operands with `%` wildcards).
var textMatchOps = map[string]bool{
	"contains":    true,
	"notContains": true,
}

// BuildFilter converts a structured filter tree (the root group) into a single
// PocketBase filter expression string, for example:
//
//	status = "active" && (total > 10 || urgent = true)
//
// It returns an empty string when there are no valid conditions.
func BuildFilter(root *FilterNode) string {
	if root == nil {
		return ""
	}
	expr, ok := renderGroupBody(root)
	if !ok {
		return ""
	}
	return expr
}

// renderGroupBody renders a group's children joined by its connector (&& / ||).
// Nested groups are wrapped in parentheses. It returns ok=false when the group
// has no usable conditions.
func renderGroupBody(group *FilterNode) (string, bool) {
	parts := make([]string, 0, len(group.Children))
	for i := range group.Children {
		child := group.Children[i]
		if child.Kind == "group" {
			if body, ok := renderGroupBody(&child); ok {
				parts = append(parts, "("+body+")")
			}
			continue
		}
		if cond, ok := renderCondition(child); ok {
			parts = append(parts, cond)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	sep := " && "
	if strings.EqualFold(group.Connector, "or") {
		sep = " || "
	}
	return strings.Join(parts, sep), true
}

// renderCondition renders a single condition to a PocketBase filter fragment
// (`field OP value`). Conditions without an attribute are skipped (ok=false).
func renderCondition(c FilterNode) (string, bool) {
	attr := strings.TrimSpace(c.Attribute)
	if attr == "" {
		return "", false
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "equal"
	}

	switch op {
	case "isNull":
		// PocketBase represents "no value" as null (also matches empty/zero).
		return attr + " = null", true
	case "isNotNull":
		return attr + " != null", true
	}

	sym, ok := operatorSymbols[op]
	if !ok {
		// Unknown operator: fall back to equality so the query still runs.
		sym = "="
	}
	return attr + " " + sym + " " + formatValue(op, c.Value), true
}

// formatValue renders a condition value as a PocketBase filter literal. Text
// match operators (`~`/`!~`) always quote the value as a string; equality and
// comparison operators emit bool/number literals when the value looks like one,
// otherwise a quoted string.
func formatValue(op, value string) string {
	if textMatchOps[op] {
		return quote(value)
	}
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "true":
		return "true"
	case "false":
		return "false"
	case "null":
		return "null"
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return trimmed
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return trimmed
	}
	return quote(value)
}

// quote wraps a string in single quotes, escaping backslashes and single quotes
// so the value is a safe PocketBase string literal.
func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

// sortParam renders the structured sort items into the PocketBase `sort`
// parameter, e.g. `-created,name`. Descending fields are prefixed with `-`.
func sortParam(items []SortItem) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		attr := strings.TrimSpace(s.Attribute)
		if attr == "" {
			continue
		}
		if strings.EqualFold(s.Direction, "desc") {
			parts = append(parts, "-"+attr)
		} else {
			parts = append(parts, attr)
		}
	}
	return strings.Join(parts, ",")
}

// fieldsParam renders the PocketBase `fields` parameter from a comma-separated
// list of field names, or returns ok=false when the list is empty.
//
// Unless system fields are being hidden, the identity fields (id, created,
// updated) are appended so the frame keeps its identity columns even when the
// user only picked user fields. PocketBase ignores unknown field names in the
// projection, so appending created/updated is safe for collections that omit
// them.
func fieldsParam(fields string, hideSystemFields bool) (string, bool) {
	keys := make([]string, 0)
	seen := map[string]bool{}
	add := func(k string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	for _, f := range strings.Split(fields, ",") {
		add(strings.TrimSpace(f))
	}
	if len(keys) == 0 {
		return "", false
	}
	if !hideSystemFields {
		for _, id := range identityColumns {
			add(id)
		}
	}
	return strings.Join(keys, ","), true
}
