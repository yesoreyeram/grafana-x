import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type DirectusQueryType = 'records' | 'count';

export interface DirectusQuery extends DataQuery {
  queryType: DirectusQueryType;
  /** Directus collection name. */
  collectionId?: string;
  /** JSON-serialized structured filter tree compiled server-side into a Directus filter object. */
  filterTree?: string;
  /** Comma-separated list of field names to include. */
  fields?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Maximum number of records to return. 0 returns all. */
  limit?: number;
  /** Number of records to skip (offset-based pagination). */
  offset?: number;
  /** Directus search parameter (full-text search). */
  search?: string;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<DirectusQuery> = {
  queryType: 'records',
  limit: 0,
  offset: 0,
};

export interface DirectusDataSourceOptions extends DataSourceJsonData {
  /** Directus API base URL. Required, since Directus is self-hosted. */
  baseURL?: string;
  /** Optional default collection name. */
  defaultCollectionId?: string;
}

export interface DirectusSecureJsonData {
  apiToken?: string;
}

export interface CollectionInfo {
  collection: string;
  name?: string;
  icon?: string;
  note?: string;
}

export interface FieldInfo {
  field: string;
  type: string;
  schema?: Record<string, unknown>;
  meta?: Record<string, unknown>;
}
