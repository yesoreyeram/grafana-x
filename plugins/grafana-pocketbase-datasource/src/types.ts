import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type PocketBaseQueryType = 'records' | 'count';

export interface PocketBaseQuery extends DataQuery {
  queryType: PocketBaseQueryType;
  /** PocketBase collection id or name. */
  collectionId?: string;
  /** JSON-serialized structured filter tree compiled server-side into a PocketBase filter expression. */
  filterTree?: string;
  /** Optional raw PocketBase filter expression. Takes precedence over filterTree when set. */
  rawFilter?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Comma-separated list of field names to include (compiled into the `fields` parameter). */
  fields?: string;
  /** When true, drop the PocketBase system fields (id, collectionId, collectionName, created, updated) from the result. */
  hideSystemFields?: boolean;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<PocketBaseQuery> = {
  queryType: 'records',
  limit: 0,
};

export type AuthMode = 'superuser' | 'user' | 'token';

/**
 * Non-secret options stored in jsonData.
 */
export interface PocketBaseDataSourceOptions extends DataSourceJsonData {
  /** PocketBase base URL, e.g. http://127.0.0.1:8090. */
  url?: string;
  /** Authentication strategy: superuser (default), user, or token. */
  authMode?: AuthMode;
  /** Superuser/user email (or username) used for password auth. */
  identity?: string;
  /** Auth collection for user-mode auth (defaults to `users`). Ignored for superuser/token. */
  authCollection?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface PocketBaseSecureJsonData {
  /** Password for superuser/user auth. */
  password?: string;
  /** Pre-issued auth token for token auth. */
  authToken?: string;
}

export interface CollectionInfo {
  id: string;
  name: string;
  type?: string;
}

export interface FieldInfo {
  name: string;
  type?: string;
}
