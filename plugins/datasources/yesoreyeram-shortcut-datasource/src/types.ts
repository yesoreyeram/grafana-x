import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type ShortcutQueryType = 'stories' | 'count';

export type StoryType = 'feature' | 'bug' | 'chore';

/**
 * How a date filter is sourced:
 * - 'any'       no date filter (default)
 * - 'dashboard' use the panel/dashboard time range, applied to one date field
 * - 'custom'    use the manually entered created/updated/deadline bounds
 */
export type ShortcutDateMode = 'any' | 'dashboard' | 'custom';

/** Which Shortcut search date operator the dashboard range applies to. */
export type ShortcutDateField = 'created' | 'updated' | 'deadline';

/** Archived constraint applied to the search. */
export type ShortcutArchived = 'any' | 'only' | 'exclude';

/** Amount of detail returned per story by the search endpoint. */
export type ShortcutDetail = 'full' | 'slim';

export interface ShortcutQuery extends DataQuery {
  queryType: ShortcutQueryType;

  /** Raw Shortcut search query (operators + free text), combined (AND) with the structured filters. */
  query?: string;

  // --- Structured filters (compiled into the search query string) ---
  /** Story type: feature / bug / chore (type:). */
  storyType?: StoryType | '';
  /** Project names (project:). */
  projects?: string[];
  /** Workflow state names (state:). */
  workflowStates?: string[];
  /** Epic name (epic:). */
  epic?: string;
  /** Iteration name (iteration:). */
  iteration?: string;
  /** Label names (label:). */
  labels?: string[];
  /** Owner mention names (owner:). */
  owners?: string[];
  /** Team names (team:). */
  teams?: string[];
  /** Archived constraint. */
  archived?: ShortcutArchived;

  // --- Date filters ---
  /** Date filter source. */
  dateMode?: ShortcutDateMode;
  /** Date field the dashboard range applies to. */
  dateField?: ShortcutDateField;
  createdAfter?: string;
  createdBefore?: string;
  updatedAfter?: string;
  updatedBefore?: string;
  deadlineAfter?: string;
  deadlineBefore?: string;

  /** Story detail level. */
  detail?: ShortcutDetail;
  /** Columns to return. Empty returns the default story field catalog. */
  fields?: string[];
  /** Maximum number of records. 0 returns all (auto-paginated, capped at 1000 by the API). */
  limit?: number;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<ShortcutQuery> = {
  queryType: 'stories',
  dateMode: 'any',
  dateField: 'created',
  archived: 'any',
  detail: 'full',
  limit: 0,
};

/** Non-secret options stored in jsonData. */
export interface ShortcutDataSourceOptions extends DataSourceJsonData {
  /** API host override. Defaults to https://api.app.shortcut.com. */
  baseURL?: string;
}

/** Secret options. Only used while editing; never sent back to the browser. */
export interface ShortcutSecureJsonData {
  apiToken?: string;
}

export interface ProjectInfo {
  id: number;
  name: string;
}

export interface EpicInfo {
  id: number;
  name: string;
}

export interface IterationInfo {
  id: number;
  name: string;
}

export interface MemberInfo {
  id: string;
  name: string;
  mention_name: string;
}

export interface TeamInfo {
  id: string;
  name: string;
  mention_name?: string;
}

export interface LabelInfo {
  id: number;
  name: string;
}

export interface WorkflowStateInfo {
  id: number;
  name: string;
  type: string;
}

export const DATE_MODE_OPTIONS: Array<{ label: string; value: ShortcutDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by date' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range (from/to)" },
  { label: 'Custom', value: 'custom', description: 'Enter explicit after/before bounds' },
];

export const DATE_FIELD_OPTIONS: Array<{ label: string; value: ShortcutDateField; description: string }> = [
  { label: 'Created', value: 'created', description: 'created: operator' },
  { label: 'Updated', value: 'updated', description: 'updated: operator' },
  { label: 'Deadline', value: 'deadline', description: 'due: operator' },
];

export const ARCHIVED_OPTIONS: Array<{ label: string; value: ShortcutArchived; description: string }> = [
  { label: 'Any', value: 'any', description: 'Do not constrain on archived state' },
  { label: 'Only archived', value: 'only', description: 'is:archived' },
  { label: 'Exclude archived', value: 'exclude', description: '!is:archived' },
];

export const DETAIL_OPTIONS: Array<{ label: string; value: ShortcutDetail; description: string }> = [
  { label: 'Full', value: 'full', description: 'All fields incl. description (Shortcut default)' },
  { label: 'Slim', value: 'slim', description: 'Omit descriptions/comments; lighter payloads' },
];

export const STORY_TYPE_OPTIONS: Array<{ label: string; value: StoryType }> = [
  { label: 'Feature', value: 'feature' },
  { label: 'Bug', value: 'bug' },
  { label: 'Chore', value: 'chore' },
];
