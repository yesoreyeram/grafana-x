import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  LinearQuery,
  LinearDataSourceOptions,
  DEFAULT_QUERY,
  TeamInfo,
  StateInfo,
  LabelInfo,
  ProjectInfo,
  UserInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<LinearQuery, LinearDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<LinearDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<LinearQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: LinearQuery, scopedVars: ScopedVars): LinearQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);
    // For multi-value lists, interpolate each entry. A multi-value variable
    // expands to comma-separated tokens (csv format), which we then split so the
    // backend receives a flat list.
    const replaceList = (values?: string[]) =>
      values
        ?.flatMap((v) => templateSrv.replace(v, scopedVars, 'csv').split(','))
        .map((v) => v.trim())
        .filter((v) => v.length > 0);

    return {
      ...query,
      teamId: replace(query.teamId),
      states: replaceList(query.states),
      assignees: replaceList(query.assignees),
      labels: replaceList(query.labels),
      projects: replaceList(query.projects),
      creator: replace(query.creator),
      searchQuery: replace(query.searchQuery),
      createdAfter: replace(query.createdAfter),
      createdBefore: replace(query.createdBefore),
      updatedAfter: replace(query.updatedAfter),
      updatedBefore: replace(query.updatedBefore),
      rawQuery: replace(query.rawQuery),
      rawVariables: replace(query.rawVariables),
    };
  }

  filterQuery(query: LinearQuery): boolean {
    // Raw queries need a document; predefined queries are always runnable.
    if (query.queryType === 'raw') {
      return !!query.rawQuery && query.rawQuery.trim().length > 0;
    }
    return true;
  }

  /** Fetch the workspace teams via the backend resource handler. */
  async getTeams(): Promise<TeamInfo[]> {
    const res = await this.getResource('teams');
    return (res?.teams ?? []) as TeamInfo[];
  }

  /** Fetch workflow states (optionally for a team) via the backend resource handler. */
  async getStates(teamId?: string): Promise<StateInfo[]> {
    const res = await this.getResource('states', teamId ? { teamId } : undefined);
    return (res?.states ?? []) as StateInfo[];
  }

  /** Fetch labels (optionally for a team) via the backend resource handler. */
  async getLabels(teamId?: string): Promise<LabelInfo[]> {
    const res = await this.getResource('labels', teamId ? { teamId } : undefined);
    return (res?.labels ?? []) as LabelInfo[];
  }

  /** Fetch the workspace projects via the backend resource handler. */
  async getProjects(): Promise<ProjectInfo[]> {
    const res = await this.getResource('projects');
    return (res?.projects ?? []) as ProjectInfo[];
  }

  /** Fetch the workspace users via the backend resource handler. */
  async getUsers(): Promise<UserInfo[]> {
    const res = await this.getResource('users');
    return (res?.users ?? []) as UserInfo[];
  }

  /** Fetch the selectable issue field names via the backend resource handler. */
  async getIssueFields(): Promise<string[]> {
    const res = await this.getResource('issuefields');
    return (res?.fields ?? []) as string[];
  }
}
