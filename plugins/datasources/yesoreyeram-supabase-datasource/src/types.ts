import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type SupabaseQueryType = 'records' | 'count';

export interface SupabaseQuery extends DataQuery {
  queryType: SupabaseQueryType;
  /** Supabase table name to query. */
  tableId?: string;
  /** Comma-separated list of columns to select (e.g. "id,name,created_at"). */
  select?: string;
  /** JSON-serialized structured filter tree compiled server-side into PostgREST query params. */
  filterTree?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Maximum number of rows. 0 returns all (auto-paginated). */
  limit?: number;
  /** Number of rows to skip. */
  offset?: number;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<SupabaseQuery> = {
  queryType: 'records',
  limit: 0,
  offset: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface SupabaseDataSourceOptions extends DataSourceJsonData {
  /** Supabase project URL. e.g. https://xxx.supabase.co/rest/v1 */
  apiUrl?: string;
  /** Optional Postgres schema (sent via the Accept-Profile header). Default: public. */
  schema?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface SupabaseSecureJsonData {
  /** The anon or service_role key. Sent as both `apikey` header and `Authorization: Bearer`. */
  serviceKey?: string;
}

export interface TableInfo {
  id: string;
  title: string;
}
