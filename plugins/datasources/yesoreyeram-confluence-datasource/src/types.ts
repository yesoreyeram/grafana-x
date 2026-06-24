import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type ConfluenceQueryType = 'pages' | 'blogposts' | 'search' | 'count';

export interface ConfluenceQuery extends DataQuery {
  queryType: ConfluenceQueryType;
  /** Numeric space id to scope pages/blogposts (and the default count). */
  spaceId?: string;
  /** CQL string for `search` queries (also scopes `count` when set). */
  cql?: string;
  /** Sort order for pages/blogposts, e.g. `-modified-date`, `title`. */
  sort?: string;
  /** Comma-separated list of columns to return. Empty returns all columns. */
  fields?: string;
  /** Optional starting pagination cursor (advanced). */
  cursor?: string;
  /** Max records to return. 0 = all (auto-paginated). */
  limit?: number;
}

export const DEFAULT_QUERY: Partial<ConfluenceQuery> = {
  queryType: 'pages',
  limit: 0,
};

export type ConfluenceAuthMode = 'basic' | 'bearer';

/**
 * Non-secret options stored in jsonData.
 */
export interface ConfluenceDataSourceOptions extends DataSourceJsonData {
  /** Wiki base URL, e.g. https://your-site.atlassian.net/wiki (Cloud). */
  baseURL?: string;
  /** Authentication mode: Basic (email + API token) or Bearer (OAuth2 / PAT). */
  authMode?: ConfluenceAuthMode;
  /** Atlassian account email (Basic auth only). */
  email?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface ConfluenceSecureJsonData {
  /** Atlassian API token (Basic auth). */
  apiToken?: string;
  /** OAuth2 access token or Data Center Personal Access Token (Bearer auth). */
  bearerToken?: string;
}

/** Space returned by the backend resource handler. */
export interface SpaceInfo {
  id: string;
  key: string;
  name: string;
  type: string;
  status: string;
}

export const QUERY_TYPE_OPTIONS: Array<{ label: string; value: ConfluenceQueryType; description: string }> = [
  { label: 'Pages', value: 'pages', description: 'List pages (optionally scoped to a space)' },
  { label: 'Blog posts', value: 'blogposts', description: 'List blog posts (optionally scoped to a space)' },
  { label: 'Search (CQL)', value: 'search', description: 'Run a Confluence Query Language search' },
  { label: 'Count', value: 'count', description: 'Number of matching pages (or CQL results)' },
];

export const SORT_OPTIONS: Array<{ label: string; value: string }> = [
  { label: 'Created date (newest first)', value: '-created-date' },
  { label: 'Created date (oldest first)', value: 'created-date' },
  { label: 'Modified date (newest first)', value: '-modified-date' },
  { label: 'Modified date (oldest first)', value: 'modified-date' },
  { label: 'Title (A→Z)', value: 'title' },
  { label: 'Title (Z→A)', value: '-title' },
  { label: 'ID (ascending)', value: 'id' },
  { label: 'ID (descending)', value: '-id' },
];
