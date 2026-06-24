import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type StrapiQueryType = 'records' | 'count';

export interface StrapiQuery extends DataQuery {
  queryType: StrapiQueryType;
  /** Strapi content type api ID (plural name, e.g. "articles"). */
  contentTypeId?: string;
  /** JSON-serialized structured filter tree compiled server-side into Strapi filter params. */
  filterTree?: string;
  /** Comma-separated list of field names to include (flattened from attributes). */
  fields?: string;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Page number for pagination. Starts at 1. */
  page?: number;
  /** Number of records per page. */
  pageSize?: number;
  /** Relations to populate (comma-separated). */
  populate?: string;
}

export const DEFAULT_QUERY: Partial<StrapiQuery> = {
  queryType: 'records',
  page: 1,
  pageSize: 25,
};

export type StrapiApiVersion = 'v4' | 'v5';

export interface StrapiDataSourceOptions extends DataSourceJsonData {
  /** Strapi API base URL. Required, since Strapi is self-hosted. */
  baseURL?: string;
  /** Optional default content type api ID. */
  defaultContentTypeId?: string;
  /**
   * Strapi major version. Selects the expected response shape (v4 nests fields
   * under `attributes`; v5 is flat with a `documentId`). Defaults to 'v5'. The
   * backend also auto-detects the shape per record, so this is a hint.
   */
  apiVersion?: StrapiApiVersion;
}

export interface StrapiSecureJsonData {
  apiToken?: string;
}

export interface ContentTypeInfo {
  uid: string;
  apiID: string;
  displayName: string;
  pluralName: string;
}

export interface FieldInfo {
  field: string;
  type: string;
}
