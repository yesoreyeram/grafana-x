package plugin

import (
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// defaultSearchQuery is sent when no structured filters or free text are
// provided. The Shortcut search endpoint requires a non-empty query; "is:story"
// matches all stories (subject to the 1000-result cap).
const defaultSearchQuery = "is:story"

// storyFieldNames is the curated catalog of selectable story columns (after
// flattening). These mirror the scalar fields produced by flattenStory for a
// Shortcut search story. When a query selects no fields, this catalog is used as
// the default column set so the frame stays readable instead of dumping every
// nested array (branches, commits, comments, …) the API returns.
var storyFieldNames = []string{
	"id",
	"name",
	"story_type",
	"description",
	"workflow_state_id",
	"epic_id",
	"iteration_id",
	"project_id",
	"group_id",
	"requested_by_id",
	"owner_ids",
	"label_ids",
	"estimate",
	"archived",
	"started",
	"completed",
	"blocked",
	"blocker",
	"num_tasks_completed",
	"position",
	"created_at",
	"updated_at",
	"started_at",
	"completed_at",
	"deadline",
	"moved_at",
	"app_url",
}

// StoryFieldNames returns the sorted catalog of selectable story field names,
// used to populate the fields multi-select in the query editor.
func StoryFieldNames() []string {
	names := make([]string, len(storyFieldNames))
	copy(names, storyFieldNames)
	sort.Strings(names)
	return names
}

// storyTypeOptions lists the valid Shortcut story types.
var storyTypeOptions = []string{"feature", "bug", "chore"}

// StoryTypeOptions returns the known Shortcut story types.
func StoryTypeOptions() []string {
	out := make([]string, len(storyTypeOptions))
	copy(out, storyTypeOptions)
	return out
}

// effectiveFields returns the field selection to apply to the frame: the
// query's selection when non-empty, otherwise the default story catalog.
func effectiveFields(fields []string) []string {
	if cleaned := nonEmpty(fields); len(cleaned) > 0 {
		return cleaned
	}
	return storyFieldNames
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

// buildSearchQuery compiles a QueryModel's structured filters and free text into
// a Shortcut search query string.
//
// Shortcut search uses AND logic across all operators (OR is not supported), and
// matches by NAME (or mention name for owners), not numeric id. Repeated
// operators are emitted verbatim — useful for labels/owners (a story can have
// several) but typically yielding no results for single-valued relations such as
// project/state (a story has one), which is a documented search limitation.
func buildSearchQuery(q QueryModel) string {
	var terms []string

	if free := strings.TrimSpace(q.Query); free != "" {
		terms = append(terms, free)
	}
	if t := strings.TrimSpace(q.StoryType); t != "" {
		terms = append(terms, "type:"+quoteTerm(t))
	}
	for _, p := range nonEmpty(q.Projects) {
		terms = append(terms, "project:"+quoteTerm(p))
	}
	for _, s := range nonEmpty(q.WorkflowStates) {
		terms = append(terms, "state:"+quoteTerm(s))
	}
	if e := strings.TrimSpace(q.Epic); e != "" {
		terms = append(terms, "epic:"+quoteTerm(e))
	}
	if it := strings.TrimSpace(q.Iteration); it != "" {
		terms = append(terms, "iteration:"+quoteTerm(it))
	}
	for _, l := range nonEmpty(q.Labels) {
		terms = append(terms, "label:"+quoteTerm(l))
	}
	for _, o := range nonEmpty(q.Owners) {
		// owner: takes a mention name; it never contains spaces but quote
		// defensively in case of custom values.
		terms = append(terms, "owner:"+quoteTerm(o))
	}
	for _, tm := range nonEmpty(q.Teams) {
		terms = append(terms, "team:"+quoteTerm(tm))
	}

	switch q.Archived {
	case archivedOnly:
		terms = append(terms, "is:archived")
	case archivedExclude:
		terms = append(terms, "!is:archived")
	}

	terms = append(terms, dateTerms(q)...)

	query := strings.TrimSpace(strings.Join(terms, " "))
	if query == "" {
		return defaultSearchQuery
	}
	return query
}

// dateTerms returns the created:/updated:/due: search terms for the query's date
// configuration.
func dateTerms(q QueryModel) []string {
	switch q.DateMode {
	case dateModeDashboard:
		r := dashboardRange(q.TimeRange)
		if r == "" {
			return nil
		}
		return []string{dateOperator(q.DateField) + ":" + r}
	case dateModeCustom:
		var out []string
		if r := dateRange(q.CreatedAfter, q.CreatedBefore); r != "" {
			out = append(out, "created:"+r)
		}
		if r := dateRange(q.UpdatedAfter, q.UpdatedBefore); r != "" {
			out = append(out, "updated:"+r)
		}
		if r := dateRange(q.DeadlineAfter, q.DeadlineBefore); r != "" {
			out = append(out, "due:"+r)
		}
		return out
	default:
		return nil
	}
}

// dateOperator maps a DateField value to its Shortcut search operator name.
func dateOperator(field string) string {
	switch field {
	case dateFieldUpdated:
		return "updated"
	case dateFieldDeadline:
		return "due"
	default:
		return "created"
	}
}

// dashboardRange formats a panel time range as a Shortcut date range
// (YYYY-MM-DD..YYYY-MM-DD). Returns "" when either bound is missing.
func dashboardRange(tr backend.TimeRange) string {
	if tr.From.IsZero() || tr.To.IsZero() {
		return ""
	}
	return tr.From.UTC().Format("2006-01-02") + ".." + tr.To.UTC().Format("2006-01-02")
}

// dateRange builds a Shortcut date range from after/before bounds. An open side
// is represented with "*". Returns "" when neither bound is set. Date terms such
// as "today"/"yesterday" are passed through unchanged.
func dateRange(after, before string) string {
	a := toSearchDate(after)
	b := toSearchDate(before)
	switch {
	case a != "" && b != "":
		return a + ".." + b
	case a != "":
		return a + "..*"
	case b != "":
		return "*.." + b
	default:
		return ""
	}
}

// toSearchDate normalises a date bound to the YYYY-MM-DD form Shortcut search
// expects. An ISO-8601 timestamp is reduced to its date part; bare date terms
// (today/yesterday/tomorrow) and already-short dates pass through.
func toSearchDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

// quoteTerm wraps a search operator value in double quotes when it contains
// whitespace (Shortcut requires quoting for multi-word values). Embedded double
// quotes are stripped because the search grammar does not support escaping.
func quoteTerm(v string) string {
	v = strings.ReplaceAll(strings.TrimSpace(v), `"`, "")
	if strings.ContainsAny(v, " \t") {
		return `"` + v + `"`
	}
	return v
}
