import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type AppwriteQueryType = 'documents' | 'count';

export interface AppwriteQuery extends DataQuery {
  queryType: AppwriteQueryType;
  /** Appwrite database id. Overrides the configured database id. */
  databaseId?: string;
  /** Appwrite collection id. */
  collectionId?: string;
  /** JSON-serialized structured filter tree compiled server-side into Appwrite queries. */
  filterTree?: string;
  /** Optional raw Appwrite query strings (one per line). Takes precedence over filterTree when set. */
  rawQueries?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Comma-separated list of attribute keys to include (compiled into a `select` query). */
  attributes?: string;
  /** When true, drop the Appwrite system fields ($id, $permissions, $collectionId, ...) from the result. */
  hideSystemFields?: boolean;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<AppwriteQuery> = {
  queryType: 'documents',
  limit: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface AppwriteDataSourceOptions extends DataSourceJsonData {
  /** Appwrite API endpoint, including the /v1 suffix. Defaults to https://cloud.appwrite.io/v1. */
  endpoint?: string;
  /** Appwrite project id, sent in the X-Appwrite-Project header. */
  projectId?: string;
  /** Optional default Appwrite database id. */
  databaseId?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface AppwriteSecureJsonData {
  apiKey?: string;
}

export interface DatabaseInfo {
  id: string;
  name: string;
}

export interface CollectionInfo {
  id: string;
  name: string;
}

export interface AttributeInfo {
  key: string;
  type?: string;
}
