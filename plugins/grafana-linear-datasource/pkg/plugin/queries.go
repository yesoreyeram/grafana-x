package plugin

import (
	"sort"
	"strings"
)

// This file holds the predefined GraphQL documents for each entity connection.
// Each query exposes a consistent set of variables: $first, $after, $orderBy and
// (for filterable types) $filter. The selected fields are deliberately a useful,
// flat subset of each entity; users needing more should use the raw query type.

// issueFieldSelections maps a logical issue field name (the output column) to its
// GraphQL selection fragment. Nested relations are flattened to scalars by the
// frame builder, so e.g. selecting "state" yields a `state { name type }` block
// that flattens to the state name.
var issueFieldSelections = map[string]string{
	"id":            "id",
	"identifier":    "identifier",
	"title":         "title",
	"description":   "description",
	"priority":      "priority",
	"priorityLabel": "priorityLabel",
	"estimate":      "estimate",
	"url":           "url",
	"branchName":    "branchName",
	"createdAt":     "createdAt",
	"updatedAt":     "updatedAt",
	"startedAt":     "startedAt",
	"completedAt":   "completedAt",
	"canceledAt":    "canceledAt",
	"archivedAt":    "archivedAt",
	"dueDate":       "dueDate",
	"state":         "state { name type }",
	"assignee":      "assignee { name email }",
	"creator":       "creator { name email }",
	"team":          "team { key name }",
	"project":       "project { name }",
	"cycle":         "cycle { number name }",
	"parent":        "parent { identifier }",
	"labels":        "labels { nodes { name } }",
}

// defaultIssueFields is the field set used when the query does not specify fields.
var defaultIssueFields = []string{
	"identifier", "title", "priority", "priorityLabel", "estimate", "url",
	"createdAt", "updatedAt", "completedAt", "canceledAt", "dueDate",
	"state", "assignee", "creator", "team", "project", "cycle", "labels",
}

// IssueFieldNames returns the sorted catalog of selectable issue field names,
// used to populate the fields multi-select in the query editor.
func IssueFieldNames() []string {
	names := make([]string, 0, len(issueFieldSelections))
	for name := range issueFieldSelections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// buildIssuesQuery assembles the issues GraphQL document with the requested
// field selection set. Unknown field names are ignored; an empty/nil selection
// falls back to the default field set. `id` is always included so rows are
// stable even when the user selects a minimal set.
func buildIssuesQuery(fields []string) string {
	selected := normalizeIssueFields(fields)

	selections := make([]string, 0, len(selected)+1)
	seen := map[string]bool{}
	// Always include id for row stability.
	selections = append(selections, issueFieldSelections["id"])
	seen["id"] = true
	for _, f := range selected {
		if seen[f] {
			continue
		}
		if frag, ok := issueFieldSelections[f]; ok {
			selections = append(selections, frag)
			seen[f] = true
		}
	}

	selectionSet := strings.Join(selections, "\n      ")
	return `query Issues($first: Int!, $after: String, $orderBy: PaginationOrderBy, $filter: IssueFilter, $includeArchived: Boolean) {
  issues(first: $first, after: $after, orderBy: $orderBy, filter: $filter, includeArchived: $includeArchived) {
    nodes {
      ` + selectionSet + `
    }
    pageInfo { hasNextPage endCursor }
  }
}`
}

// normalizeIssueFields returns the valid requested fields, or the default set
// when none are valid/provided.
func normalizeIssueFields(fields []string) []string {
	valid := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := issueFieldSelections[f]; ok {
			valid = append(valid, f)
		}
	}
	if len(valid) == 0 {
		return defaultIssueFields
	}
	return valid
}

const projectsQuery = `query Projects($first: Int!, $after: String, $orderBy: PaginationOrderBy, $filter: ProjectFilter) {
  projects(first: $first, after: $after, orderBy: $orderBy, filter: $filter) {
    nodes {
      id
      name
      description
      state
      progress
      url
      startDate
      targetDate
      createdAt
      updatedAt
      completedAt
      canceledAt
      lead { name email }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const teamsQuery = `query Teams($first: Int!, $after: String, $orderBy: PaginationOrderBy) {
  teams(first: $first, after: $after, orderBy: $orderBy) {
    nodes {
      id
      key
      name
      description
      private
      createdAt
      updatedAt
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const usersQuery = `query Users($first: Int!, $after: String, $orderBy: PaginationOrderBy) {
  users(first: $first, after: $after, orderBy: $orderBy) {
    nodes {
      id
      name
      displayName
      email
      active
      admin
      guest
      createdAt
      updatedAt
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const cyclesQuery = `query Cycles($first: Int!, $after: String, $orderBy: PaginationOrderBy, $filter: CycleFilter) {
  cycles(first: $first, after: $after, orderBy: $orderBy, filter: $filter) {
    nodes {
      id
      number
      name
      startsAt
      endsAt
      completedAt
      progress
      createdAt
      updatedAt
      team { key name }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// buildConnectionQuery returns the GraphQL document for a predefined query type.
// The issues query is built dynamically from the requested field set; the others
// are static.
func buildConnectionQuery(q QueryModel) string {
	switch q.QueryType {
	case queryTypeProjects:
		return projectsQuery
	case queryTypeTeams:
		return teamsQuery
	case queryTypeUsers:
		return usersQuery
	case queryTypeCycles:
		return cyclesQuery
	default:
		return buildIssuesQuery(q.Fields)
	}
}
