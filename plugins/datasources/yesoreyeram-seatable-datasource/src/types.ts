import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type SeaTableQueryType = 'records' | 'count' | 'sql';

export interface SeaTableQuery extends DataQuery {
  queryType: SeaTableQueryType;
  /** Table name within the base. Required for records/count queries. */
  tableName?: string;
  /**
   * Optional view name. Only applied for plain record listings (no
   * filter/sort/fields), which use the rows endpoint; filtered/sorted/projected
   * listings run via SQL, which has no view concept.
   */
  viewName?: string;
  /** JSON-serialized structured filter tree compiled server-side into a SQL WHERE clause. */
  filterTree?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Comma-separated list of column names to include. Empty returns all columns. */
  fields?: string;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;
  /** Raw SeaTable SQL statement, used when queryType is "sql". */
  sql?: string;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<SeaTableQuery> = {
  queryType: 'records',
  limit: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface SeaTableDataSourceOptions extends DataSourceJsonData {
  /** SeaTable server URL. Defaults to https://cloud.seatable.io. */
  serverURL?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface SeaTableSecureJsonData {
  /** SeaTable Base API Token (exchanged server-side for a Base-Token). */
  apiToken?: string;
}

export interface ColumnInfo {
  key: string;
  name: string;
  type: string;
}

export interface TableInfo {
  name: string;
  columns: ColumnInfo[];
}
