package plugin

import (
	"strconv"
	"strings"
)

// searchQueryField maps an entity to the field that the free-text SearchQuery is
// matched against (with the contains operator `~`).
var searchQueryField = map[string]string{
	queryTypeContacts:      "email",
	queryTypeCompanies:     "name",
	queryTypeConversations: "source.body",
	queryTypeTickets:       "title",
}

// BuildSearchQuery compiles the structured pickers and generic filter rows into
// an Intercom Search API `query` object. Conditions are combined with AND. It
// returns nil when no criteria are present (the caller then falls back to a
// list endpoint, or to a match-all query for search-only entities).
//
// The shape of the returned value follows the Intercom Search API:
//   - a single condition is returned bare: {field, operator, value}
//   - multiple conditions are wrapped: {operator: "AND", value: [...]}
func BuildSearchQuery(q QueryModel, entity string) map[string]any {
	conditions := make([]map[string]any, 0)

	if v := strings.TrimSpace(q.StatusFilter); v != "" {
		conditions = append(conditions, condition("state", "=", v))
	}
	if v := strings.TrimSpace(q.Role); v != "" {
		conditions = append(conditions, condition("role", "=", v))
	}
	if v := strings.TrimSpace(q.AssigneeID); v != "" {
		conditions = append(conditions, condition("admin_assignee_id", "=", coerceValue(v)))
	}
	if v := strings.TrimSpace(q.TeamID); v != "" {
		conditions = append(conditions, condition("team_assignee_id", "=", coerceValue(v)))
	}
	if v := strings.TrimSpace(q.TagID); v != "" {
		conditions = append(conditions, condition("tag_ids", "=", coerceValue(v)))
	}
	if v := strings.TrimSpace(q.SearchQuery); v != "" {
		field := searchQueryField[entity]
		if field == "" {
			field = "name"
		}
		conditions = append(conditions, condition(field, "~", v))
	}

	for _, f := range q.Filters {
		field := strings.TrimSpace(f.Field)
		if field == "" {
			continue
		}
		op := strings.TrimSpace(f.Operator)
		if op == "" {
			op = "="
		}
		conditions = append(conditions, condition(field, op, coerceValue(f.Value)))
	}

	switch len(conditions) {
	case 0:
		return nil
	case 1:
		return conditions[0]
	default:
		return map[string]any{"operator": "AND", "value": conditions}
	}
}

// matchAllQuery returns a benign query that matches every record. Used for
// search-only entities (tickets) when the user supplied no criteria, since the
// Intercom Search API requires a query body.
func matchAllQuery() map[string]any {
	return condition("created_at", ">", 0)
}

func condition(field, operator string, value any) map[string]any {
	return map[string]any{"field": field, "operator": operator, "value": value}
}

// buildSort converts the sort string (e.g. `-created_at`) into the Intercom
// Search API sort object {field, order}. Returns nil when no sort is set.
func buildSort(sort string) map[string]any {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return nil
	}
	order := "ascending"
	if strings.HasPrefix(sort, "-") {
		order = "descending"
		sort = strings.TrimSpace(sort[1:])
	}
	if sort == "" {
		return nil
	}
	return map[string]any{"field": sort, "order": order}
}

// buildSortParams converts the sort string into the (field, order) query-string
// pair used by the list endpoints. order is "asc"/"desc". Empty field means no
// sort.
func buildSortParams(sort string) (string, string) {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return "", ""
	}
	order := "asc"
	if strings.HasPrefix(sort, "-") {
		order = "desc"
		sort = strings.TrimSpace(sort[1:])
	}
	return sort, order
}

// coerceValue converts a string filter value to the most specific JSON type so
// numeric and timestamp fields (e.g. created_at) are sent as numbers, which the
// Intercom Search API requires for comparison operators. Falls back to the raw
// string.
func coerceValue(v string) any {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return v
	}
	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}
	switch strings.ToLower(trimmed) {
	case "true":
		return true
	case "false":
		return false
	}
	return v
}
