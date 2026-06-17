package plugin

import (
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

// operatorArity classifies how many values an operator consumes:
//
//	"none" – unary (no value), e.g. blank/notblank
//	"list" – comma-separated tokens, e.g. in/anyof/allof
//	"single" – a single quoted value (default)
func operatorArity(op string) string {
	switch op {
	case "blank", "notblank":
		return "none"
	case "in", "anyof", "allof", "nanyof", "nallof":
		return "list"
	default:
		return "single"
	}
}

// quoteValue wraps a value in double quotes (escaping embedded quotes) so it is
// safe to use within the NocoDB `@`-prefixed where grammar even when it contains
// commas or parentheses.
func quoteValue(v string) string {
	return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
}

// BuildWhere converts a structured filter tree (the root group) into a NocoDB
// `where` clause. It returns an empty string when there are no valid conditions.
//
// quoted controls the leading `@`: NocoDB's v2 API requires the `@` prefix to
// opt into the quote-aware where parser, while the v3 API rejects it (v3 parses
// quoted values natively).
func BuildWhere(root *FilterNode, quoted bool) string {
	if root == nil {
		return ""
	}
	inner := serializeGroup(root)
	if inner == "" {
		return ""
	}
	if quoted {
		return "@" + inner
	}
	return inner
}

func serializeNode(node FilterNode) string {
	if node.Kind == "group" {
		return serializeGroup(&node)
	}
	return serializeCondition(node)
}

func serializeGroup(group *FilterNode) string {
	parts := make([]string, 0, len(group.Children))
	for _, child := range group.Children {
		if s := serializeNode(child); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	joiner := "~and"
	if strings.EqualFold(group.Connector, "or") {
		joiner = "~or"
	}
	return "(" + strings.Join(parts, joiner) + ")"
}

func serializeCondition(c FilterNode) string {
	field := strings.TrimSpace(c.Field)
	if field == "" {
		return ""
	}
	op := strings.TrimSpace(c.Op)
	if op == "" {
		op = "eq"
	}
	fieldToken := quoteValue(field)

	switch operatorArity(op) {
	case "none":
		return "(" + fieldToken + "," + op + ")"
	case "list":
		tokens := make([]string, 0)
		for _, t := range strings.Split(c.Value, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tokens = append(tokens, t)
			}
		}
		if len(tokens) == 0 {
			return ""
		}
		return "(" + fieldToken + "," + op + "," + strings.Join(tokens, ",") + ")"
	default: // single
		v := strings.TrimSpace(c.Value)
		if v == "" {
			return ""
		}
		return "(" + fieldToken + "," + op + "," + quoteValue(v) + ")"
	}
}
