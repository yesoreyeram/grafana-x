import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type AttioQueryType = 'records' | 'count';

export interface AttioQuery extends DataQuery {
  queryType: AttioQueryType;
  /** Attio object api_slug (e.g. people, companies, deals). */
  objectId?: string;
  /** JSON-serialized structured filter tree compiled server-side into an Attio filter object. */
  filterTree?: string;
  /** Comma-separated list of attribute slugs to include. Empty returns all. */
  fields?: string;
  /** When true, hide synthetic system columns (_record_id, _created_at) from the returned frame. */
  hideSystemFields?: boolean;
  /** JSON-serialized structured sort items. */
  sort?: string;
  /** Maximum number of records to return. 0 returns all (auto-paginated). */
  limit?: number;
  /** Number of records to skip (offset-based pagination). */
  offset?: number;
}

export const DEFAULT_QUERY: Partial<AttioQuery> = {
  queryType: 'records',
  limit: 0,
  offset: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface AttioDataSourceOptions extends DataSourceJsonData {
  /** Root URL of the Attio API. Defaults to https://api.attio.com. */
  baseURL?: string;
  /** Optional default object api_slug used to prefill the query editor. */
  defaultObjectId?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface AttioSecureJsonData {
  apiToken?: string;
}

export interface ObjectInfo {
  api_slug: string;
  singular_noun?: string;
  plural_noun?: string;
}

export interface AttributeInfo {
  api_slug: string;
  title?: string;
  /** Raw Attio attribute type (e.g. text, number, date, status). */
  type?: string;
  is_required?: boolean;
}
