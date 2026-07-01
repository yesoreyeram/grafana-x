import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type GristQueryType = 'records' | 'count' | 'sql';

export interface GristQuery extends DataQuery {
  queryType: GristQueryType;
  /** Grist doc id. Overrides the configured default doc id. */
  docId?: string;
  /** Grist table id or name. */
  tableId?: string;
  /**
   * JSON-serialized structured filter tree. Simple equality/membership filters
   * compile to the Grist records `filter` param; richer operators compile to a
   * parameterized SQL WHERE clause server-side.
   */
  filterTree?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Comma-separated list of field names to include. Forces the SQL path. */
  fields?: string;
  /** Maximum number of records to return (0 = no limit, Grist returns all). */
  limit?: number;
  /** Raw read-only Grist SQL SELECT statement (queryType = 'sql'). */
  sql?: string;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<GristQuery> = {
  queryType: 'records',
  limit: 0,
};

export interface GristDataSourceOptions extends DataSourceJsonData {
  /**
   * Grist API base URL. Cloud team sites: https://{team}.getgrist.com.
   * Self-hosted: the instance URL (e.g. http://localhost:8484). A trailing
   * `/api` is accepted and normalised away by the backend.
   */
  baseURL?: string;
  /** Optional default Grist doc id. */
  docId?: string;
}

export interface GristSecureJsonData {
  apiKey?: string;
}

export interface DocInfo {
  id: string;
  title: string;
}

export interface TableInfo {
  id: string;
  title: string;
}

export interface FieldInfo {
  title: string;
  type?: string;
}
