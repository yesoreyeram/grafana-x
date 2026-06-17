import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type ClickUpQueryType = 'tasks' | 'spaces' | 'folders' | 'lists' | 'teams' | 'raw';

export type ClickUpOrderBy = 'created' | 'updated' | 'due_date' | 'id';

/**
 * How a date filter (created / updated / due) is sourced:
 * - 'any'       no time filter (default)
 * - 'dashboard' use the panel/dashboard time range (from/to)
 * - 'custom'    use the manually entered after/before bounds
 */
export type ClickUpDateMode = 'any' | 'dashboard' | 'custom';

export interface ClickUpQuery extends DataQuery {
  queryType: ClickUpQueryType;

  // --- Scope (which part of the hierarchy to read) ---
  /** Workspace (team) ID. Required for tasks/spaces queries. */
  teamId?: string;
  /** Space ID. Scopes folders/lists/tasks queries. Optional. */
  spaceId?: string;
  /** Folder ID. Scopes lists/tasks queries. Optional. */
  folderId?: string;
  /** List ID. Scopes tasks queries to a single List. Optional. */
  listId?: string;

  // --- Task filters (used when queryType is "tasks") ---
  /** Status names to filter tasks by (OR'd). Optional. */
  statuses?: string[];
  /** Assignee user IDs to filter tasks by (OR'd). Optional. */
  assignees?: string[];
  /** Tag names to filter tasks by (OR'd). Optional. */
  tags?: string[];
  /** Include closed tasks. Optional. */
  includeClosed?: boolean;
  /** Include subtasks. Optional. */
  includeSubtasks?: boolean;
  /** Include archived tasks. Optional. */
  includeArchived?: boolean;

  /** Source of the date_created filter. Defaults to 'any'. */
  createdMode?: ClickUpDateMode;
  /** Custom lower bound for date_created (ISO date-time or Unix millis). */
  createdAfter?: string;
  /** Custom upper bound for date_created (ISO date-time or Unix millis). */
  createdBefore?: string;
  /** Source of the date_updated filter. Defaults to 'any'. */
  updatedMode?: ClickUpDateMode;
  /** Custom lower bound for date_updated. */
  updatedAfter?: string;
  /** Custom upper bound for date_updated. */
  updatedBefore?: string;
  /** Source of the due_date filter. Defaults to 'any'. */
  dueMode?: ClickUpDateMode;
  /** Custom lower bound for due_date. */
  dueAfter?: string;
  /** Custom upper bound for due_date. */
  dueBefore?: string;

  /** Fields (columns) to return for tasks. Empty returns all fields. */
  fields?: string[];
  /** Task ordering. Defaults to 'created'. */
  orderBy?: ClickUpOrderBy;
  /** Reverse the order direction. Optional. */
  reverse?: boolean;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;

  // --- Raw query (used when queryType is "raw") ---
  /** REST GET path relative to the API root, e.g. "/v2/team/123/task". */
  rawPath?: string;
  /** Optional response key holding the array/object to flatten into rows. */
  rawRoot?: string;
}

export const DEFAULT_QUERY: Partial<ClickUpQuery> = {
  queryType: 'tasks',
  orderBy: 'created',
  limit: 0,
};

export type ClickUpAuthMethod = 'apiKey' | 'oauth';

/**
 * Non-secret options stored in jsonData.
 */
export interface ClickUpDataSourceOptions extends DataSourceJsonData {
  /** API root. Defaults to https://api.clickup.com/api. */
  baseURL?: string;
  /** Authentication method: personal token or OAuth access token. */
  authMethod?: ClickUpAuthMethod;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface ClickUpSecureJsonData {
  apiKey?: string;
  oauthToken?: string;
}

export interface TeamInfo {
  id: string;
  name: string;
}

export interface SpaceInfo {
  id: string;
  name: string;
}

export interface FolderInfo {
  id: string;
  name: string;
}

export interface ClickUpListInfo {
  id: string;
  name: string;
}

export interface MemberInfo {
  id: string;
  username?: string;
  email?: string;
}

/** Options for the created/updated/due date-filter mode selector. */
export const DATE_MODE_OPTIONS: Array<{ label: string; value: ClickUpDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by this date' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range (from/to)" },
  { label: 'Custom', value: 'custom', description: 'Enter explicit after/before bounds' },
];
