import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type CodaQueryType = 'rows' | 'count';

/** Coda rows `valueFormat`. `simple` returns scalar cells (arrays become comma strings). */
export type CodaValueFormat = 'simple' | 'simpleWithArrays' | 'rich';

/** Coda rows `sortBy` (the RowsSortBy enum). Empty means Coda's default (creation order). */
export type CodaSortBy = '' | 'createdAt' | 'updatedAt' | 'natural';

export interface CodaQuery extends DataQuery {
  queryType: CodaQueryType;
  /** Coda doc id. Overrides the configured default doc id. */
  docId?: string;
  /** Coda table id or name. */
  tableId?: string;
  /** Comma-separated list of column names to include. Projection is applied server-side after fetch. */
  columns?: string;
  /** Single-column equality filter: the column name (or id). */
  filterColumn?: string;
  /** Single-column equality filter: the value. */
  filterValue?: string;
  /** Advanced raw Coda `query` parameter (`<columnIdOrName>:<value>`). Takes precedence over the column filter. */
  query?: string;
  /** Row sort order (Coda RowsSortBy enum). */
  sortBy?: CodaSortBy;
  /** When true, return only visible rows/columns. Implied by sortBy=natural. */
  visibleOnly?: boolean;
  /** Cell value format. Defaults to `simple`. */
  valueFormat?: CodaValueFormat;
  /** Maximum number of rows. 0 returns all (auto-paginated). */
  limit?: number;
}

export const DEFAULT_QUERY: Partial<CodaQuery> = {
  queryType: 'rows',
  valueFormat: 'simple',
  limit: 0,
};

export interface CodaDataSourceOptions extends DataSourceJsonData {
  /** Optional default Coda doc id. */
  docId?: string;
}

export interface CodaSecureJsonData {
  apiToken?: string;
}

export interface DocInfo {
  id: string;
  title: string;
}

export interface TableInfo {
  id: string;
  title: string;
}

export interface ColumnInfo {
  title: string;
  type?: string;
}
