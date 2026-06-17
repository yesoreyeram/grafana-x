package plugin

import (
	"sort"
	"strings"
)

// Supported query types.
const (
	queryTypeWorkItems = "workitems"
	queryTypeProjects  = "projects"
	queryTypeStates    = "states"
	queryTypeLabels    = "labels"
	queryTypeCycles    = "cycles"
	queryTypeModules   = "modules"
	queryTypeMembers   = "members"
	queryTypeRaw       = "raw"
)

// workItemFieldNames is the catalog of selectable work item columns (after
// flattening). These mirror the scalar fields produced by flattenEntity in
// frame.go for a Plane work item. Selecting a subset restricts the returned
// columns; an empty selection returns them all.
//
// Nested Plane objects are flattened to a readable scalar by the frame builder
// (e.g. "state" -> the state name when expanded, "assignees" -> joined names or
// ids), so these names match the output columns rather than the raw API shape.
var workItemFieldNames = []string{
	"id",
	"name",
	"description_stripped",
	"priority",
	"sequence_id",
	"state",
	"state_group",
	"assignees",
	"labels",
	"estimate_point",
	"start_date",
	"target_date",
	"completed_at",
	"created_at",
	"updated_at",
	"created_by",
	"updated_by",
	"project",
	"parent",
	"cycle",
	"module",
	"is_draft",
	"archived_at",
}

// WorkItemFieldNames returns the sorted catalog of selectable work item field
// names, used to populate the fields multi-select in the query editor.
func WorkItemFieldNames() []string {
	names := make([]string, len(workItemFieldNames))
	copy(names, workItemFieldNames)
	sort.Strings(names)
	return names
}

// priorityOptions lists the valid Plane work item priorities (used for the
// priority multi-select hints; the filter accepts any value).
var priorityOptions = []string{"urgent", "high", "medium", "low", "none"}

// PriorityOptions returns the known Plane work item priorities.
func PriorityOptions() []string {
	out := make([]string, len(priorityOptions))
	copy(out, priorityOptions)
	return out
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

// normalizeOrderBy maps the editor value to Plane's order_by query parameter.
// Plane accepts any model field, optionally prefixed with "-" for descending.
// Defaults to "-created_at" (newest first).
func normalizeOrderBy(orderBy string) string {
	if v := strings.TrimSpace(orderBy); v != "" {
		return v
	}
	return "-created_at"
}
