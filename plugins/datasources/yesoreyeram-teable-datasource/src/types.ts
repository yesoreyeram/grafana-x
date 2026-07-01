import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type TeableQueryType = 'records' | 'count';

export interface TeableQuery extends DataQuery {
  queryType: TeableQueryType;
  /** Teable base ID (only needed by the editor to list tables). */
  baseId?: string;
  /** Table ID within the base (starts with "tbl..."). */
  tableId?: string;
  /** Optional Teable view ID (starts with "viw..."). Honours the view's filter/sort. */
  viewId?: string;
  /** JSON-serialized structured filter tree compiled server-side into the Teable JSON filter object. */
  filterTree?: string;
  /** Comma-separated list of field names to include. */
  fields?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<TeableQuery> = {
  queryType: 'records',
  limit: 0,
};

export interface TeableDataSourceOptions extends DataSourceJsonData {
  /** Teable API base URL. Required since Teable is self-hosted or cloud. */
  baseURL?: string;
  /** Optional default base ID. */
  defaultBaseId?: string;
}

export interface TeableSecureJsonData {
  apiToken?: string;
}

export interface TableInfo {
  id: string;
  name: string;
}

export interface FieldInfo {
  id: string;
  name: string;
  type: string;
  isPrimary?: boolean;
}
