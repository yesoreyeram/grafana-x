import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

import { SearchFilter } from './filter';

export type IntercomQueryType =
  | 'conversations'
  | 'contacts'
  | 'tickets'
  | 'articles'
  | 'companies'
  | 'admins'
  | 'teams'
  | 'tags'
  | 'count';

/** Entities that can be counted by the `count` query type. */
export type IntercomCountEntity = 'conversations' | 'contacts' | 'tickets' | 'articles' | 'companies';

export interface IntercomQuery extends DataQuery {
  queryType: IntercomQueryType;
  /** Entity to count when queryType === 'count'. */
  countOf?: IntercomCountEntity;
  /** Conversation state filter (open/closed/snoozed). */
  statusFilter?: string;
  /** Contact role filter (user/lead). */
  role?: string;
  /** Filter by admin assignee id (admin_assignee_id). */
  assigneeId?: string;
  /** Filter by team assignee id (team_assignee_id). */
  teamId?: string;
  /** Filter by tag id (tag_ids contains). */
  tagId?: string;
  /** Free-text contains match against the entity's primary text field. */
  searchQuery?: string;
  /** Generic Intercom Search API conditions. */
  filters?: SearchFilter[];
  /** Sort field, optionally prefixed with `-` for descending (e.g. `-created_at`). */
  sort?: string;
  /** Maximum records to return. 0 returns all (auto-paginated up to a safety cap). */
  limit?: number;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<IntercomQuery> = {
  queryType: 'conversations',
  countOf: 'conversations',
  limit: 0,
};

export type IntercomRegion = 'us' | 'eu' | 'au';

/** Non-secret options stored in jsonData. */
export interface IntercomDataSourceOptions extends DataSourceJsonData {
  /** Root URL of the Intercom API. Overrides Region when set. */
  baseURL?: string;
  /** Intercom data residency region (us/eu/au). Used to derive the base URL. */
  region?: IntercomRegion;
  /** Value of the Intercom-Version header (e.g. 2.11). */
  intercomVersion?: string;
}

/** Secret options. Only used while editing; never sent back to the browser. */
export interface IntercomSecureJsonData {
  apiToken?: string;
}

// ----- Resource types returned by the backend ---------------------------------

export interface AdminInfo {
  id: string;
  name: string;
  email: string;
}

export interface TeamInfo {
  id: string;
  name: string;
}

export interface TagInfo {
  id: string;
  name: string;
}

// ----- Static option catalogs -------------------------------------------------

export const QUERY_TYPE_OPTIONS: Array<{ label: string; value: IntercomQueryType; description: string }> = [
  { label: 'Conversations', value: 'conversations', description: 'List or search conversations' },
  { label: 'Contacts', value: 'contacts', description: 'List or search contacts (users & leads)' },
  { label: 'Tickets', value: 'tickets', description: 'Search tickets' },
  { label: 'Articles', value: 'articles', description: 'List help center articles' },
  { label: 'Companies', value: 'companies', description: 'List companies' },
  { label: 'Admins', value: 'admins', description: 'List workspace admins (teammates)' },
  { label: 'Teams', value: 'teams', description: 'List workspace teams' },
  { label: 'Tags', value: 'tags', description: 'List workspace tags' },
  { label: 'Count', value: 'count', description: 'Count records for an entity' },
];

export const COUNT_OF_OPTIONS: Array<{ label: string; value: IntercomCountEntity }> = [
  { label: 'Conversations', value: 'conversations' },
  { label: 'Contacts', value: 'contacts' },
  { label: 'Tickets', value: 'tickets' },
  { label: 'Articles', value: 'articles' },
  { label: 'Companies', value: 'companies' },
];

export const CONVERSATION_STATE_OPTIONS: Array<{ label: string; value: string }> = [
  { label: 'Any', value: '' },
  { label: 'Open', value: 'open' },
  { label: 'Closed', value: 'closed' },
  { label: 'Snoozed', value: 'snoozed' },
];

export const CONTACT_ROLE_OPTIONS: Array<{ label: string; value: string }> = [
  { label: 'Any', value: '' },
  { label: 'User', value: 'user' },
  { label: 'Lead', value: 'lead' },
];

export const REGION_OPTIONS: Array<{ label: string; value: IntercomRegion; description: string }> = [
  { label: 'US', value: 'us', description: 'https://api.intercom.io' },
  { label: 'EU', value: 'eu', description: 'https://api.eu.intercom.io' },
  { label: 'AU', value: 'au', description: 'https://api.au.intercom.io' },
];
