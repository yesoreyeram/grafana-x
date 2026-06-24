import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type TodoistQueryType = 'tasks' | 'count';

export interface TodoistQuery extends DataQuery {
  queryType: TodoistQueryType;

  /** Project ID to scope tasks to. Optional. */
  projectId?: string;
  /** Section ID to scope tasks to. Requires a project. Optional. */
  sectionId?: string;
  /** Label NAME to scope tasks to (the Todoist `label` parameter filters by name, not ID). Optional. */
  label?: string;
  /** Parent task ID to fetch sub-tasks of. Optional. */
  parentId?: string;
  /**
   * Todoist filter query (e.g. "today | overdue", "#Work & p1"). When set, the
   * query is sent to the dedicated /tasks/filter endpoint and the project /
   * section / label / parent scope is ignored. Optional.
   */
  filter?: string;
  /** IETF language tag used to parse the filter string (e.g. "en", "de"). Only used with filter. Optional. */
  lang?: string;
  /** Maximum number of records to return (and, for count, tasks to scan). 0 returns all (auto-paginated). */
  limit?: number;
}

export const DEFAULT_QUERY: Partial<TodoistQuery> = {
  queryType: 'tasks',
  limit: 0,
};

/** Non-secret options stored in jsonData. */
export interface TodoistDataSourceOptions extends DataSourceJsonData {
  /** API root. Defaults to https://api.todoist.com/api/v1. Overridable for a proxy/gateway. */
  baseURL?: string;
}

/** Secret options. Only used while editing; never sent back to the browser. */
export interface TodoistSecureJsonData {
  apiToken?: string;
}

/** A Todoist project (id + name) used to populate the editor dropdown. */
export interface ProjectInfo {
  id: string;
  name: string;
}

/** A Todoist section (id + name) used to populate the editor dropdown. */
export interface SectionInfo {
  id: string;
  name: string;
}

/** A Todoist label (id + name) used to populate the editor dropdown. The query stores the label NAME. */
export interface LabelInfo {
  id: string;
  name: string;
}
