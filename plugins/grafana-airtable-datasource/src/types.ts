import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type AirtableQueryType = 'records' | 'count';

export interface AirtableQuery extends DataQuery {
  queryType: AirtableQueryType;
  /** Airtable base id (starts with "app..."). Overrides the configured base id. */
  baseId?: string;
  /** Airtable table id (starts with "tbl...") or table name. */
  tableId?: string;
  /** Optional Airtable view id (starts with "viw...") or name. */
  viewId?: string;
  /** JSON-serialized structured filter tree compiled server-side into a filterByFormula. */
  filterTree?: string;
  /** Optional raw Airtable formula. Takes precedence over filterTree when set. */
  filterByFormula?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Comma-separated list of field names to include. */
  fields?: string;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<AirtableQuery> = {
  queryType: 'records',
  limit: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface AirtableDataSourceOptions extends DataSourceJsonData {
  /** Airtable API base URL. Defaults to https://api.airtable.com. */
  baseURL?: string;
  /** Optional default Airtable base id (starts with "app..."). */
  baseId?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface AirtableSecureJsonData {
  apiToken?: string;
}

export interface BaseInfo {
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

export interface ViewInfo {
  id: string;
  title: string;
}
