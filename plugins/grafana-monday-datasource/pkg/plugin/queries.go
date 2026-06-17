package plugin

import "strings"

// This file holds the predefined GraphQL documents for each monday.com query
// type. monday.com is a single GraphQL endpoint; items live under boards and are
// paginated with cursor-based `items_page` / `next_items_page`, while boards,
// users, workspaces and tags use page/limit pagination.

// Supported query types.
const (
	queryTypeItems      = "items"
	queryTypeBoards     = "boards"
	queryTypeGroups     = "groups"
	queryTypeUsers      = "users"
	queryTypeWorkspaces = "workspaces"
	queryTypeTags       = "tags"
	queryTypeRaw        = "raw"
)

// Board/item lifecycle states.
const (
	stateActive   = "active"
	stateAll      = "all"
	stateArchived = "archived"
	stateDeleted  = "deleted"
)

// validState reports whether the supplied state is one monday.com accepts.
func validState(state string) bool {
	switch state {
	case stateActive, stateAll, stateArchived, stateDeleted:
		return true
	default:
		return false
	}
}

// columnValuesBlock returns the column_values selection, optionally restricted to
// specific column ids via the GraphQL `ids` argument. The column `type` is always
// selected so type-aware conversion (e.g. checkbox -> boolean) is possible.
func columnValuesBlock(indent string, columnIDs []string) string {
	idsArg := ""
	if len(columnIDs) > 0 {
		idsArg = "(ids: " + jsonStringArray(columnIDs) + ")"
	}
	return "\n" + indent + "column_values" + idsArg + ` {
` + indent + `  id
` + indent + `  type
` + indent + `  column { id title }
` + indent + `  text
` + indent + `  value
` + indent + `}`
}

// jsonStringArray renders ids as a GraphQL list of strings, e.g. ["status","date"].
func jsonStringArray(ids []string) string {
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		// Column ids are simple slugs; escape quotes defensively.
		quoted = append(quoted, `"`+strings.ReplaceAll(id, `"`, `\"`)+`"`)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// buildItemsQuery fetches the first page of items for a board. It uses
// `query_params` (rules / order) and `items_page(limit, cursor, query_params)`.
// The cursor is returned for subsequent pages via buildNextItemsQuery. When
// columnIDs is non-empty only those column values are requested.
func buildItemsQuery(withColumns bool, columnIDs []string) string {
	columnBlock := ""
	if withColumns {
		columnBlock = columnValuesBlock("        ", columnIDs)
	}
	return `query Items($boardIds: [ID!], $limit: Int!, $cursor: String, $queryParams: ItemsQuery) {
  boards(ids: $boardIds) {
    id
    name
    items_page(limit: $limit, cursor: $cursor, query_params: $queryParams) {
      cursor
      items {
        id
        name
        state
        created_at
        updated_at
        group { id title }
        board { id name }` + columnBlock + `
      }
    }
  }
}`
}

// buildNextItemsQuery fetches subsequent item pages from a cursor.
func buildNextItemsQuery(withColumns bool, columnIDs []string) string {
	columnBlock := ""
	if withColumns {
		columnBlock = columnValuesBlock("      ", columnIDs)
	}
	return `query NextItems($cursor: String!, $limit: Int!) {
  next_items_page(cursor: $cursor, limit: $limit) {
    cursor
    items {
      id
      name
      state
      created_at
      updated_at
      group { id title }
      board { id name }` + columnBlock + `
    }
  }
}`
}

const boardsQuery = `query Boards($ids: [ID!], $limit: Int!, $page: Int!, $workspaceIds: [ID], $state: State) {
  boards(ids: $ids, limit: $limit, page: $page, workspace_ids: $workspaceIds, state: $state) {
    id
    name
    description
    state
    board_kind
    items_count
    updated_at
    workspace { id name }
    owners { id name }
  }
}`

const groupsQuery = `query Groups($boardIds: [ID!]) {
  boards(ids: $boardIds) {
    id
    name
    groups {
      id
      title
      color
      position
      archived
      deleted
    }
  }
}`

const usersQuery = `query Users($limit: Int!, $page: Int!) {
  users(limit: $limit, page: $page) {
    id
    name
    email
    enabled
    is_admin
    is_guest
    is_view_only
    created_at
    title
  }
}`

const workspacesQuery = `query Workspaces($limit: Int!, $page: Int!, $state: State) {
  workspaces(limit: $limit, page: $page, state: $state) {
    id
    name
    kind
    description
    created_at
  }
}`

const tagsQuery = `query Tags {
  tags {
    id
    name
    color
  }
}`
