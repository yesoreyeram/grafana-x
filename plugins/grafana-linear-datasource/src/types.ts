import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type LinearQueryType = 'issues' | 'projects' | 'teams' | 'users' | 'cycles' | 'raw';

export type LinearOrderBy = 'createdAt' | 'updatedAt';

/**
 * How a date filter (created / updated) is sourced:
 * - 'any'       no time filter (default)
 * - 'dashboard' use the panel/dashboard time range (from/to)
 * - 'custom'    use the manually entered after/before bounds
 */
export type LinearDateMode = 'any' | 'dashboard' | 'custom';

export interface LinearQuery extends DataQuery {
  queryType: LinearQueryType;
  /** Team UUID to filter issues/cycles by. Optional. */
  teamId?: string;
  /** Workflow state names to filter issues by (OR'd). Optional. */
  states?: string[];
  /** Assignee emails or names to filter issues by (OR'd). Optional. */
  assignees?: string[];
  /** Label names to filter issues by (issue has any of them). Optional. */
  labels?: string[];
  /** Priorities to filter issues by (0=None,1=Urgent,2=High,3=Medium,4=Low). Optional. */
  priorities?: number[];
  /** Project names to filter issues by (OR'd). Optional. */
  projects?: string[];
  /** Creator email or name to filter issues by. Optional. */
  creator?: string;
  /** Free-text filter applied to issue titles. Optional. */
  searchQuery?: string;
  /** Source of the createdAt filter. Defaults to 'any'. */
  createdMode?: LinearDateMode;
  /** Custom lower bound for createdAt (ISO date-time). Used when createdMode is 'custom'. */
  createdAfter?: string;
  /** Custom upper bound for createdAt (ISO date-time). Used when createdMode is 'custom'. */
  createdBefore?: string;
  /** Source of the updatedAt filter. Defaults to 'any'. */
  updatedMode?: LinearDateMode;
  /** Custom lower bound for updatedAt (ISO date-time). Used when updatedMode is 'custom'. */
  updatedAfter?: string;
  /** Custom upper bound for updatedAt (ISO date-time). Used when updatedMode is 'custom'. */
  updatedBefore?: string;
  /** Include archived issues in the results. Optional. */
  includeArchived?: boolean;
  /** Fields (columns) to return for issues. Empty returns the default set. */
  fields?: string[];
  /** Connection ordering. Defaults to createdAt. */
  orderBy?: LinearOrderBy;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;
  /** Raw GraphQL document, used when queryType is "raw". */
  rawQuery?: string;
  /** Optional JSON object of variables for the raw query. */
  rawVariables?: string;
}

export const DEFAULT_QUERY: Partial<LinearQuery> = {
  queryType: 'issues',
  orderBy: 'createdAt',
  limit: 0,
};

export type LinearAuthMethod = 'apiKey' | 'oauth';

/**
 * Non-secret options stored in jsonData.
 */
export interface LinearDataSourceOptions extends DataSourceJsonData {
  /** GraphQL endpoint. Defaults to https://api.linear.app/graphql. */
  baseURL?: string;
  /** Authentication method: personal API key or OAuth access token. */
  authMethod?: LinearAuthMethod;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface LinearSecureJsonData {
  apiKey?: string;
  oauthToken?: string;
}

export interface TeamInfo {
  id: string;
  key: string;
  name: string;
}

export interface StateInfo {
  name: string;
  type?: string;
  teamKey?: string;
}

export interface LabelInfo {
  name: string;
}

export interface ProjectInfo {
  id: string;
  name: string;
}

export interface UserInfo {
  name: string;
  email?: string;
}

export interface FieldInfo {
  /** Field name as it appears in the GraphQL selection / output column. */
  name: string;
  /** Optional human description. */
  description?: string;
}

/** Priority options shared by the editor (Linear priority is 0-4). */
export const PRIORITY_OPTIONS: Array<{ label: string; value: number }> = [
  { label: 'No priority', value: 0 },
  { label: 'Urgent', value: 1 },
  { label: 'High', value: 2 },
  { label: 'Medium', value: 3 },
  { label: 'Low', value: 4 },
];

/** Options for the created/updated date-filter mode selector. */
export const DATE_MODE_OPTIONS: Array<{ label: string; value: LinearDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by this date' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range (from/to)" },
  { label: 'Custom', value: 'custom', description: 'Enter explicit after/before bounds' },
];
