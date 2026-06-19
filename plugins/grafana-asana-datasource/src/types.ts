import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type AsanaQueryType = 'tasks' | 'projects' | 'sections' | 'workspaces' | 'teams' | 'users' | 'tags' | 'raw';

/**
 * How the modified-since date filter is sourced:
 * - 'any'       no time filter (default)
 * - 'dashboard' use the panel/dashboard time range (from)
 * - 'custom'    use the manually entered ISO-8601 bound
 */
export type AsanaDateMode = 'any' | 'dashboard' | 'custom';

export interface AsanaQuery extends DataQuery {
  queryType: AsanaQueryType;

  // --- Scope (which part of the hierarchy to read) ---
  /** Workspace/organization gid. Required for projects/teams/users/tags and assignee-scoped tasks. */
  workspace?: string;
  /** Team gid (organizations only). Scopes the projects list. Optional. */
  team?: string;
  /** Project gid. Scopes sections and tasks queries. Optional. */
  project?: string;
  /** Section gid. Scopes tasks queries to a single section. Optional. */
  section?: string;
  /** Assignee user gid (or "me"). Scopes tasks queries; requires workspace. Optional. */
  assignee?: string;

  // --- Task filters (used when queryType is "tasks") ---
  /** Return only incomplete tasks (completed_since=now). Optional. */
  incompleteOnly?: boolean;
  /** Source of the modified_since filter. Defaults to 'any'. */
  modifiedMode?: AsanaDateMode;
  /** Custom lower bound for modified_at (ISO-8601). */
  modifiedSince?: string;

  // --- Projects filter (used when queryType is "projects") ---
  /** Include archived projects. Optional. */
  includeArchived?: boolean;

  /** Fields (columns) to return for tasks. Empty returns the default field set. */
  fields?: string[];
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;

  // --- Raw query (used when queryType is "raw") ---
  /** REST GET path relative to the API root, e.g. "/workspaces". */
  rawPath?: string;
  /** Optional response key holding the array/object to flatten into rows. */
  rawRoot?: string;
}

export const DEFAULT_QUERY: Partial<AsanaQuery> = {
  queryType: 'tasks',
  limit: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface AsanaDataSourceOptions extends DataSourceJsonData {
  /** API root. Defaults to https://app.asana.com/api/1.0. */
  baseURL?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface AsanaSecureJsonData {
  apiKey?: string;
}

/** A lightweight Asana resource (gid + name) used to populate the editor dropdowns. */
export interface AsanaResource {
  gid: string;
  name: string;
}

/** Options for the modified-since date-filter mode selector. */
export const DATE_MODE_OPTIONS: Array<{ label: string; value: AsanaDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by modified time' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range (from)" },
  { label: 'Custom', value: 'custom', description: 'Enter an explicit ISO-8601 bound' },
];
