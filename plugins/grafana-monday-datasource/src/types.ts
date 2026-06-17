import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type MondayQueryType = 'items' | 'boards' | 'groups' | 'users' | 'workspaces' | 'tags' | 'raw';

export type MondayState = 'active' | 'all' | 'archived' | 'deleted';

export type MondayOrderDir = 'asc' | 'desc';

export type MondayAggregation = 'count' | 'count_distinct' | 'sum' | 'avg' | 'min' | 'max';

export interface MondayQuery extends DataQuery {
  queryType: MondayQueryType;
  /** Board IDs to query. Required for items/groups; optional filter for boards. */
  boardIds?: string[];
  /** Group IDs within the board(s) to filter items by. Optional. */
  groupIds?: string[];
  /** Workspace IDs to filter the boards query by. Optional. */
  workspaceIds?: string[];
  /** Column IDs to restrict the items column_values selection to. Optional. */
  columnIds?: string[];
  /** Free-text filter applied to item names. Optional. */
  searchQuery?: string;
  /** Lifecycle state for boards/items/workspaces. Defaults to 'active'. */
  state?: MondayState;
  /** Include flattened column values for items. Defaults to true. */
  includeColumnValues?: boolean;
  /** Hide monday.com's built-in/system column values for items. Defaults to false. */
  hideSystemColumns?: boolean;
  /** Column ID to order items by. Optional. */
  orderBy?: string;
  /** Order direction for items. Defaults to 'asc'. */
  orderDir?: MondayOrderDir;
  /** Board column ID to group items by (e.g. "status", "person"). When set, the query uses monday's server-side aggregate API. Optional. */
  groupBy?: string;
  /** Aggregation applied within each group. Defaults to 'count'. Used only when groupBy is set. */
  aggregation?: MondayAggregation;
  /** Board column ID whose values are aggregated for sum/avg/min/max and count_distinct. Used only when groupBy is set. */
  aggregationColumn?: string;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;
  /** Raw GraphQL document, used when queryType is "raw". */
  rawQuery?: string;
  /** Optional JSON object of variables for the raw query. */
  rawVariables?: string;
}

export const DEFAULT_QUERY: Partial<MondayQuery> = {
  queryType: 'items',
  state: 'active',
  includeColumnValues: true,
  limit: 0,
};

export type MondayAuthMethod = 'apiKey' | 'oauth';

/**
 * Non-secret options stored in jsonData.
 */
export interface MondayDataSourceOptions extends DataSourceJsonData {
  /** GraphQL endpoint. Defaults to https://api.monday.com/v2. */
  baseURL?: string;
  /** Authentication method: personal API token or OAuth access token. */
  authMethod?: MondayAuthMethod;
  /** Optional monday.com API version, sent as the API-Version header. */
  apiVersion?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface MondaySecureJsonData {
  apiToken?: string;
  oauthToken?: string;
}

export interface BoardInfo {
  id: string;
  name: string;
}

export interface GroupInfo {
  id: string;
  title: string;
}

export interface ColumnInfo {
  id: string;
  title: string;
  type?: string;
}

export interface WorkspaceInfo {
  id: string;
  name: string;
}

/** Options for the lifecycle state selector. */
export const STATE_OPTIONS: Array<{ label: string; value: MondayState; description: string }> = [
  { label: 'Active', value: 'active', description: 'Only active (non-archived) records' },
  { label: 'All', value: 'all', description: 'Active, archived and deleted records' },
  { label: 'Archived', value: 'archived', description: 'Only archived records' },
  { label: 'Deleted', value: 'deleted', description: 'Only deleted records' },
];

/** Options for the aggregation selector (used when grouping items). */
export const AGGREGATION_OPTIONS: Array<{ label: string; value: MondayAggregation; description: string }> = [
  { label: 'Count', value: 'count', description: 'Number of items in each group' },
  { label: 'Count distinct', value: 'count_distinct', description: 'Distinct non-empty values of the value column' },
  { label: 'Sum', value: 'sum', description: 'Sum of the value column' },
  { label: 'Average', value: 'avg', description: 'Average of the value column' },
  { label: 'Min', value: 'min', description: 'Minimum of the value column' },
  { label: 'Max', value: 'max', description: 'Maximum of the value column' },
];

/** Aggregations that require a value column. */
export const AGGREGATIONS_NEEDING_COLUMN: MondayAggregation[] = ['count_distinct', 'sum', 'avg', 'min', 'max'];

/** Options for the items order direction selector. */
export const ORDER_DIR_OPTIONS: Array<{ label: string; value: MondayOrderDir }> = [
  { label: 'Ascending', value: 'asc' },
  { label: 'Descending', value: 'desc' },
];
