package plugin

import (
	"sort"
	"strings"
)

// Supported query types.
const (
	queryTypeTasks   = "tasks"
	queryTypeSpaces  = "spaces"
	queryTypeFolders = "folders"
	queryTypeLists   = "lists"
	queryTypeTeams   = "teams"
	queryTypeRaw     = "raw"
)

// taskFieldNames is the catalog of selectable task columns (after flattening).
// These mirror the scalar fields produced by flattenTask in frame.go. Selecting
// a subset restricts the returned columns; an empty selection returns them all.
//
// Nested ClickUp objects are flattened to a readable scalar by the frame builder
// (e.g. "status" -> the status name, "assignees" -> comma-joined usernames), so
// these names match the output columns rather than the raw API shape.
var taskFieldNames = []string{
	"id",
	"custom_id",
	"name",
	"text_content",
	"description",
	"status",
	"status_type",
	"orderindex",
	"date_created",
	"date_updated",
	"date_closed",
	"date_done",
	"start_date",
	"due_date",
	"archived",
	"creator",
	"assignees",
	"watchers",
	"tags",
	"parent",
	"priority",
	"points",
	"time_estimate",
	"time_spent",
	"url",
	"list",
	"folder",
	"space",
}

// TaskFieldNames returns the sorted catalog of selectable task field names, used
// to populate the fields multi-select in the query editor.
func TaskFieldNames() []string {
	names := make([]string, len(taskFieldNames))
	copy(names, taskFieldNames)
	sort.Strings(names)
	return names
}

// nonEmpty returns the trimmed, non-empty entries of a string slice.
func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// normalizeOrderBy maps the editor value to ClickUp's order_by query parameter.
// ClickUp supports: id, created, updated, due_date. Defaults to "created".
func normalizeOrderBy(orderBy string) string {
	switch strings.TrimSpace(orderBy) {
	case "id":
		return "id"
	case "updated":
		return "updated"
	case "due_date":
		return "due_date"
	default:
		return "created"
	}
}
