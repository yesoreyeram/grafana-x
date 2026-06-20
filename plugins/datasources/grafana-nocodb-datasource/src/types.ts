import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type NocoDBQueryType = 'records' | 'count';

export interface NocoDBQuery extends DataQuery {
  queryType: NocoDBQueryType;
  tableId?: string;
  /** NocoDB base id (prefixed with p). Required for the v3 data API. */
  baseId?: string;
  viewId?: string;
  /** Optional raw NocoDB where override. The where is normally built server-side from filterTree. */
  where?: string;
  /** JSON-serialized structured filter tree built server-side into a where clause. */
  filterTree?: string;
  sort?: string;
  fields?: string;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<NocoDBQuery> = {
  queryType: 'records',
  limit: 0,
};

export type NocoDBPlatform = 'cloud' | 'selfhosted';
export type NocoDBApiVersion = 'v2' | 'v3';

/**
 * Non-secret options stored in jsonData.
 */
export interface NocoDBDataSourceOptions extends DataSourceJsonData {
  /** Deployment: NocoDB Cloud (app.nocodb.com) or a self-hosted instance. */
  platform?: NocoDBPlatform;
  baseURL?: string;
  /** NocoDB data API version to use for record queries. */
  apiVersion?: NocoDBApiVersion;
  /** Optional default base id used to populate the table dropdown. */
  baseId?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface NocoDBSecureJsonData {
  apiToken?: string;
}

export interface TableInfo {
  id: string;
  title: string;
  baseId?: string;
  baseTitle?: string;
}

export interface FieldInfo {
  title: string;
  type?: string;
}

export interface ViewInfo {
  id: string;
  title: string;
}
