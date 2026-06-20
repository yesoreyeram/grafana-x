package plugin

import (
	"sort"
	"strings"
)

// Supported query types.
const (
	queryTypeTasks      = "tasks"
	queryTypeProjects   = "projects"
	queryTypeSections   = "sections"
	queryTypeWorkspaces = "workspaces"
	queryTypeTeams      = "teams"
	queryTypeUsers      = "users"
	queryTypeTags       = "tags"
	queryTypeRaw        = "raw"
)

// taskFieldPaths maps a friendly task field name (matching the flattened output
// column) to the Asana `opt_fields` path requested from the API. Asana returns
// only compact records (gid, name, resource_type) unless `opt_fields` is set, so
// the field selection is applied server-side.
//
// Nested relations are requested by their `.name` (e.g. assignee.name) and
// flattened back to the friendly column name (assignee) by frame.go.
var taskFieldPaths = map[string]string{
	"gid":             "gid",
	"name":            "name",
	"resource_type":   "resource_type",
	"completed":       "completed",
	"completed_at":    "completed_at",
	"created_at":      "created_at",
	"modified_at":     "modified_at",
	"start_on":        "start_on",
	"due_on":          "due_on",
	"due_at":          "due_at",
	"assignee":        "assignee.name",
	"assignee_status": "assignee_status",
	"projects":        "projects.name",
	"parent":          "parent.name",
	"tags":            "tags.name",
	"num_subtasks":    "num_subtasks",
	"notes":           "notes",
	"permalink_url":   "permalink_url",
	"custom_fields":   taskCustomFieldOptFields,
}

// taskCustomFieldOptFields requests the parts of each custom field value object
// needed to reduce it to a single typed column (see frame.go::addCustomFields).
// Asana returns custom field values only when these are opted in.
const taskCustomFieldOptFields = "custom_fields.name," +
	"custom_fields.display_value," +
	"custom_fields.type," +
	"custom_fields.number_value," +
	"custom_fields.text_value," +
	"custom_fields.enum_value.name," +
	"custom_fields.multi_enum_values.name," +
	"custom_fields.date_value.date," +
	"custom_fields.date_value.date_time," +
	"custom_fields.people_value.name"

// taskFieldNames is the ordered catalog of selectable task columns (after
// flattening). The order doubles as the default field set requested when the
// user has not chosen specific fields.
var taskFieldNames = []string{
	"gid",
	"name",
	"resource_type",
	"completed",
	"completed_at",
	"created_at",
	"modified_at",
	"start_on",
	"due_on",
	"due_at",
	"assignee",
	"assignee_status",
	"projects",
	"parent",
	"tags",
	"num_subtasks",
	"notes",
	"permalink_url",
	"custom_fields",
}

// optFields for the non-task list query types. These enrich the compact records
// Asana returns by default so the frames carry useful columns.
const (
	projectOptFields   = "gid,name,resource_type,archived,color,created_at,modified_at,start_on,due_on,public,notes,owner.name,team.name,current_status.text,permalink_url"
	sectionOptFields   = "gid,name,resource_type,created_at"
	workspaceOptFields = "gid,name,resource_type,is_organization"
	userOptFields      = "gid,name,resource_type,email"
	tagOptFields       = "gid,name,resource_type,color,notes"
)

// TaskFieldNames returns the sorted catalog of selectable task field names, used
// to populate the fields multi-select in the query editor.
func TaskFieldNames() []string {
	names := make([]string, len(taskFieldNames))
	copy(names, taskFieldNames)
	sort.Strings(names)
	return names
}

// taskOptFields returns the comma-separated Asana opt_fields value for a task
// request. When fields is empty the full default catalog is requested. Unknown
// names are passed through unchanged so callers can request custom paths.
func taskOptFields(fields []string) string {
	selected := nonEmpty(fields)
	if len(selected) == 0 {
		selected = taskFieldNames
	}
	paths := make([]string, 0, len(selected))
	seen := map[string]bool{}
	for _, f := range selected {
		path := f
		if mapped, ok := taskFieldPaths[f]; ok {
			path = mapped
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return strings.Join(paths, ",")
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
