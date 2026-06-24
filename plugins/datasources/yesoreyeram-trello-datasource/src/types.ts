import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type TrelloQueryType = 'cards' | 'count';

export type TrelloCardFilter = 'all' | 'open' | 'closed';

/**
 * How the card creation-date filter is sourced:
 * - 'any'       no date filter (default)
 * - 'dashboard' use the panel/dashboard time range (from/to)
 * - 'custom'    use the manually entered after/before bounds
 *
 * Trello's card endpoints only support filtering by creation date (via the
 * before/since cursor that operates on the card id's embedded timestamp), so
 * there is a single creation-date filter rather than created/updated/due.
 */
export type TrelloDateMode = 'any' | 'dashboard' | 'custom';

export interface TrelloQuery extends DataQuery {
  queryType: TrelloQueryType;
  /** Board to read cards from. Required. */
  boardId?: string;
  /** Optional list on the board to narrow the query to. */
  listId?: string;
  /** Card visibility filter: all (default), open, or closed. */
  cardFilter?: TrelloCardFilter;
  /** Member ids to filter cards by (matches any). Applied client-side. */
  memberIds?: string[];
  /** Label ids to filter cards by (matches any). Applied client-side. */
  labelIds?: string[];
  /** Source of the card creation-date filter. Defaults to 'any'. */
  createdMode?: TrelloDateMode;
  /** Custom lower bound for the creation date (ISO-8601 or Unix millis). */
  createdAfter?: string;
  /** Custom upper bound for the creation date (ISO-8601 or Unix millis). */
  createdBefore?: string;
  /** Columns (after flattening) to return. Empty returns all. */
  fields?: string[];
  /** Maximum number of cards to return. 0 returns all (auto-paginated). */
  limit?: number;
}

export const DEFAULT_QUERY: Partial<TrelloQuery> = {
  queryType: 'cards',
  cardFilter: 'all',
  createdMode: 'any',
  limit: 0,
};

export interface TrelloDataSourceOptions extends DataSourceJsonData {}

export interface TrelloSecureJsonData {
  apiKey?: string;
  apiToken?: string;
}

export interface BoardInfo {
  id: string;
  name: string;
  desc?: string;
  shortUrl?: string;
  closed?: boolean;
}

export interface ListInfo {
  id: string;
  name: string;
  closed?: boolean;
  pos?: number;
}

export interface MemberInfo {
  id: string;
  fullName?: string;
  username?: string;
  avatarUrl?: string;
}

export interface LabelInfo {
  id: string;
  name: string;
  color?: string;
}

export const CARD_FILTER_OPTIONS: Array<{ label: string; value: TrelloCardFilter }> = [
  { label: 'All', value: 'all' },
  { label: 'Open', value: 'open' },
  { label: 'Closed', value: 'closed' },
];

/** Options for the card creation-date filter mode selector. */
export const DATE_MODE_OPTIONS: Array<{ label: string; value: TrelloDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by creation date' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range (from/to)" },
  { label: 'Custom', value: 'custom', description: 'Enter explicit after/before bounds' },
];

/** Catalog of selectable card columns (mirrors the flattened backend output). */
export const CARD_FIELD_OPTIONS: Array<{ label: string; value: string }> = [
  'id',
  'name',
  'desc',
  'closed',
  'pos',
  'shortUrl',
  'url',
  'idList',
  'idBoard',
  'idMembers',
  'labels',
  'idChecklists',
  'due',
  'dueComplete',
  'start',
  'dateCreated',
  'dateLastActivity',
  'badges_votes',
  'badges_comments',
  'badges_attachments',
  'badges_checkItems',
  'badges_checkItemsChecked',
  'customFieldItems',
].map((f) => ({ label: f, value: f }));
