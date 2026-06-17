import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type PlaneQueryType =
  | 'workitems'
  | 'projects'
  | 'states'
  | 'labels'
  | 'cycles'
  | 'modules'
  | 'members'
  | 'raw';

/**
 * How a date filter (created / updated) is sourced:
 * - 'any'       no time filter (default)
 * - 'dashboard' use the panel/dashboard time range (from/to)
 * - 'custom'    use the manually entered after/before bounds
 */
export type PlaneDateMode = 'any' | 'dashboard' | 'custom';

export interface PlaneQuery extends DataQuery {
  queryType: PlaneQueryType;

  // --- Scope (which part of the hierarchy to read) ---
  /** Workspace slug. Falls back to the data source default when empty. */
  workspaceSlug?: string;
  /** Project UUID. Required for work items / states / labels / cycles / modules. */
  projectId?: string;

  // --- Work item filters (used when queryType is "workitems") ---
  /** Priority names to filter by (urgent/high/medium/low/none). Matches any. */
  priorities?: string[];
  /** State UUIDs to filter by (matches any). */
  states?: string[];
  /** Assignee user UUIDs to filter by (matches any). */
  assignees?: string[];
  /** Label UUIDs to filter by (matches any). */
  labels?: string[];

  /** Source of the created_at filter. Defaults to 'any'. */
  createdMode?: PlaneDateMode;
  /** Custom lower bound for created_at (ISO-8601 / RFC3339). */
  createdAfter?: string;
  /** Custom upper bound for created_at. */
  createdBefore?: string;
  /** Source of the updated_at filter. Defaults to 'any'. */
  updatedMode?: PlaneDateMode;
  /** Custom lower bound for updated_at. */
  updatedAfter?: string;
  /** Custom upper bound for updated_at. */
  updatedBefore?: string;

  /** Related objects to expand inline (e.g. assignees, state, labels). */
  expand?: string[];

  /** Fields (columns) to return for work items. Empty returns all fields. */
  fields?: string[];
  /** Field to order results by; prefix with '-' for descending. */
  orderBy?: string;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;

  // --- Raw query (used when queryType is "raw") ---
  /** REST GET path relative to the API root, e.g. "/api/v1/workspaces/my-team/projects/". */
  rawPath?: string;
  /** Optional response key holding the array/object to flatten into rows. */
  rawRoot?: string;
}

export const DEFAULT_QUERY: Partial<PlaneQuery> = {
  queryType: 'workitems',
  orderBy: '-created_at',
  limit: 0,
};

export type PlaneAuthMethod = 'apiKey' | 'oauth';

/**
 * Non-secret options stored in jsonData.
 */
export interface PlaneDataSourceOptions extends DataSourceJsonData {
  /** API root. Defaults to https://api.plane.so. */
  baseURL?: string;
  /** Default workspace slug used when a query does not set one. */
  workspaceSlug?: string;
  /** Authentication method: personal API key or OAuth access token. */
  authMethod?: PlaneAuthMethod;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface PlaneSecureJsonData {
  apiKey?: string;
  oauthToken?: string;
}

export interface ProjectInfo {
  id: string;
  name: string;
  identifier?: string;
}

export interface StateInfo {
  id: string;
  name: string;
  group?: string;
}

export interface LabelInfo {
  id: string;
  name: string;
}

export interface MemberInfo {
  id: string;
  display_name?: string;
  email?: string;
}

/** Options for the created/updated date-filter mode selector. */
export const DATE_MODE_OPTIONS: Array<{ label: string; value: PlaneDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by this date' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range (from/to)" },
  { label: 'Custom', value: 'custom', description: 'Enter explicit after/before bounds' },
];

/** Options available to expand inline on work item queries. */
export const EXPAND_OPTIONS: Array<{ label: string; value: string }> = [
  { label: 'assignees', value: 'assignees' },
  { label: 'state', value: 'state' },
  { label: 'labels', value: 'labels' },
  { label: 'modules', value: 'modules' },
  { label: 'issue_cycle', value: 'issue_cycle' },
];
