import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type NotionQueryType = 'records' | 'count';

export interface NotionQuery extends DataQuery {
  queryType: NotionQueryType;
  /** Notion database id. Required for record/count queries. */
  databaseId?: string;
  /** JSON-serialized structured filter tree built server-side into a Notion filter object. */
  filterTree?: string;
  /** Comma-separated sort string (e.g. `-Created,Name`). */
  sort?: string;
  /** Comma-separated list of property names to include. Empty returns all. */
  fields?: string;
  limit?: number;
}

export const DEFAULT_QUERY: Partial<NotionQuery> = {
  queryType: 'records',
  limit: 0,
};

/**
 * Non-secret options stored in jsonData.
 */
export interface NotionDataSourceOptions extends DataSourceJsonData {
  /** Root URL of the Notion API. Defaults to https://api.notion.com. */
  baseURL?: string;
  /** Value of the Notion-Version header (e.g. 2022-06-28). */
  notionVersion?: string;
  /** Optional default database id. */
  databaseId?: string;
}

/**
 * Secret options. Only used while editing; never sent back to the browser.
 */
export interface NotionSecureJsonData {
  apiToken?: string;
}

export interface DatabaseInfo {
  id: string;
  title: string;
}

export interface PropertyInfo {
  title: string;
  /** Raw Notion property type (e.g. rich_text, number, checkbox). */
  type?: string;
  /** Logical category used by the filter builder. */
  category?: string;
}
