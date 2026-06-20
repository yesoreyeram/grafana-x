import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type BaserowQueryType = 'records' | 'count';

export interface BaserowQuery extends DataQuery {
  queryType: BaserowQueryType;
  /** Baserow table id (numeric). */
  tableId?: string;
  /** Optional Baserow view id (numeric); applies the view's filters and sorts. */
  viewId?: string;
  /** JSON-serialized structured filter tree built server-side into a Baserow `filters` tree. */
  filterTree?: string;
  /** Comma-separated list of field names (prefix - for descending). */
  sort?: string;
  /** Comma-separated list of field names to include. */
  fields?: string;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<BaserowQuery> = {
  queryType: 'records',
  limit: 0,
};

export type BaserowPlatform = 'cloud' | 'selfhosted';

/**
 * Authentication mode:
 * - `token`: a Baserow database token (scoped to a single database).
 * - `password`: email + password exchanged for a JWT (can list all databases).
 */
export type BaserowAuthMode = 'token' | 'password';

/**
 * Non-secret options stored in jsonData.
 */
export interface BaserowDataSourceOptions extends DataSourceJsonData {
  /** Deployment: Baserow Cloud (api.baserow.io) or a self-hosted instance. */
  platform?: BaserowPlatform;
  baseURL?: string;
  /** Authentication mode. Defaults to `token`. */
  authMode?: BaserowAuthMode;
  /** Baserow database (application) id used to list tables (required for token auth). */
  databaseId?: string;
  /** Baserow account email (password auth mode). */
  email?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface BaserowSecureJsonData {
  apiToken?: string;
  password?: string;
}

export interface DatabaseInfo {
  id: string;
  title: string;
  workspaceId?: string;
  workspaceName?: string;
}

export interface TableInfo {
  id: string;
  title: string;
  databaseId?: string;
}

export interface FieldInfo {
  title: string;
  type?: string;
}

export interface ViewInfo {
  id: string;
  title: string;
}
